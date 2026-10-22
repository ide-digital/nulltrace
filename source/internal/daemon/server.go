package daemon

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ide-digital/nulltrace/internal/app"
	"github.com/ide-digital/nulltrace/internal/broker"
	"github.com/ide-digital/nulltrace/internal/engine/playbook"
	"github.com/ide-digital/nulltrace/internal/nterr"
	"github.com/ide-digital/nulltrace/pkg/ipc"
)

type Server struct {
	rt    *app.Runtime
	token string
	stop  context.CancelFunc
	ln    net.Listener
	mu    sync.Mutex
}

func NewServer(rt *app.Runtime, token string, stop context.CancelFunc) *Server {
	return &Server{rt: rt, token: token, stop: stop}
}

func (s *Server) Listen() error {
	addr := s.rt.Cfg.ListenAddr
	if addr == "" {
		addr = "127.0.0.1:7738"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok && !tcp.IP.IsLoopback() {
		s.rt.Log.Warn("ipc is listening beyond localhost; only the token protects it", "addr", tcp.String())
	}
	s.rt.Log.Info("ipc listening", "addr", ln.Addr().String())
	return nil
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return s.rt.Cfg.ListenAddr
	}
	return s.ln.Addr().String()
}

func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	ln := s.ln
	s.mu.Unlock()
	if ln == nil {
		return fmt.Errorf("server not listening")
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Minute))
	br := bufio.NewReader(conn)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return
	}
	var env ipc.Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		writeReply(conn, ipc.Reply{V: ipc.ProtocolVersion, OK: false, Error: "malformed request"})
		return
	}
	rep := s.dispatch(ctx, env)
	writeReply(conn, rep)
}

func writeReply(conn net.Conn, rep ipc.Reply) {
	if rep.V == 0 {
		rep.V = ipc.ProtocolVersion
	}
	_ = json.NewEncoder(conn).Encode(rep)
}

// bind is json.Unmarshal that lets a command come with no payload at all.
func bind(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("bad payload: %w", err)
	}
	return nil
}

func (s *Server) dispatch(ctx context.Context, env ipc.Envelope) ipc.Reply {
	rep := ipc.Reply{V: ipc.ProtocolVersion, ID: env.ID}
	if subtle.ConstantTimeCompare([]byte(env.Token), []byte(s.token)) != 1 {
		rep.Error = nterr.ErrUnauthorized.Error()
		return rep
	}
	var (
		payload any
		err     error
	)
	switch env.Cmd {
	case ipc.CmdPing:
		payload = map[string]string{"pong": "ok"}
	case ipc.CmdStatus:
		payload, err = s.rt.Status(ctx)
	case ipc.CmdSnapshot:
		payload, err = s.rt.Snapshot(ctx)
	case ipc.CmdScan:
		var req ipc.ScanRequest
		if err = bind(env.Payload, &req); err == nil {
			_, payload, err = s.rt.Scan(ctx, req.EnableHIBP)
		}
	case ipc.CmdScrub:
		payload, err = s.rt.Scrub(ctx)
	case ipc.CmdExport:
		var req ipc.ExportRequest
		if err = bind(env.Payload, &req); err == nil {
			payload, err = s.rt.Export(ctx, req.Format)
		}
	case ipc.CmdActionComplete:
		var req ipc.ActionIDRequest
		if err = bind(env.Payload, &req); err == nil {
			err = s.rt.CompleteAction(ctx, req.ActionID)
			payload = map[string]string{"action_id": req.ActionID}
		}
	case ipc.CmdActionVerify:
		var req ipc.ActionIDRequest
		if err = bind(env.Payload, &req); err == nil {
			err = s.rt.VerifyRemoved(ctx, req.RecordID)
			payload = map[string]string{"record_id": req.RecordID}
		}
	case ipc.CmdIdentityAdd:
		var req ipc.IdentityAddRequest
		if err = bind(env.Payload, &req); err == nil {
			payload, err = s.rt.AddIdentity(ctx, req)
		}
	case ipc.CmdAttrAdd:
		var req ipc.AttrAddRequest
		if err = bind(env.Payload, &req); err == nil {
			err = s.rt.AddAttribute(ctx, req)
			payload = map[string]string{"ok": "1"}
		}
	case ipc.CmdSecretSet:
		var req ipc.SecretSetRequest
		if err = bind(env.Payload, &req); err == nil {
			if req.Key == "" || req.Value == "" {
				err = fmt.Errorf("secret key and value are required")
				break
			}
			err = s.rt.Store.PutSecret(ctx, req.Key, []byte(req.Value))
			payload = map[string]string{"key": req.Key}
		}
	case ipc.CmdPlaybook:
		var req ipc.PlaybookRequest
		if err = bind(env.Payload, &req); err != nil {
			break
		}
		b, ok := s.rt.Reg.Get(req.BrokerID)
		if !ok {
			row, e := s.rt.Store.GetBroker(ctx, req.BrokerID)
			if e != nil {
				err = nterr.ErrBrokerNotFound
				break
			}
			b = broker.Broker{
				ID: row.ID, Name: row.Name, Domain: row.Domain,
				OptOutURL: row.OptOutURL, ContactEmail: row.ContactEmail,
				RequiresCaptcha: row.RequiresCaptcha, Playbook: row.Playbook,
				RequiresEmailConfirmation: row.RequiresEmailConfirmation,
			}
		}
		payload = ipc.PlaybookResult{Text: playbook.Render(playbook.For(b))}
	case ipc.CmdFootprint:
		var req ipc.FootprintRequest
		if err = bind(env.Payload, &req); err == nil {
			payload, err = s.rt.Footprint(ctx, req.Username, req.Persist)
		}
	case ipc.CmdAutomate:
		var req ipc.AutomateRequest
		if err = bind(env.Payload, &req); err == nil {
			payload, err = s.rt.Automate(ctx, req.BrokerID, req.Headful)
		}
	case ipc.CmdShutdown:
		payload = map[string]string{"bye": "1"}
		// Give the reply a moment to go out before the listener closes.
		go func() {
			time.Sleep(50 * time.Millisecond)
			s.stop()
		}()
	default:
		err = fmt.Errorf("unknown command %q", env.Cmd)
	}
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	raw, mErr := json.Marshal(payload)
	if mErr != nil {
		rep.Error = mErr.Error()
		return rep
	}
	rep.OK = true
	rep.Payload = raw
	return rep
}
