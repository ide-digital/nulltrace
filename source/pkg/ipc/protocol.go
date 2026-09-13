package ipc

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
)

func Encode(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

func Decode[T any](r io.Reader) (T, error) {
	var v T
	dec := json.NewDecoder(r)
	err := dec.Decode(&v)
	return v, err
}

type Client struct {
	addr  string
	token string
}

func NewClient(addr, token string) *Client {
	return &Client{addr: addr, token: token}
}

func LoadToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(trimNL(b)), nil
}

func WriteToken(path, token string) error {
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

func RandomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func trimNL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

func (c *Client) Call(ctx context.Context, cmd string, payload any) (Reply, error) {
	var raw json.RawMessage
	var err error
	if payload != nil {
		raw, err = json.Marshal(payload)
		if err != nil {
			return Reply{}, err
		}
	}
	env := Envelope{
		V:       ProtocolVersion,
		ID:      uuid.NewString(),
		Token:   c.token,
		Cmd:     cmd,
		Payload: raw,
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return Reply{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Minute))
	}
	if err := Encode(conn, env); err != nil {
		return Reply{}, err
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return Reply{}, err
	}
	if len(line) == 0 {
		return Reply{}, fmt.Errorf("empty ipc reply")
	}
	var rep Reply
	if err := json.Unmarshal(line, &rep); err != nil {
		return Reply{}, fmt.Errorf("ipc decode: %w", err)
	}
	if !rep.OK {
		if rep.Error == "" {
			rep.Error = "ipc command failed"
		}
		return rep, fmt.Errorf("%s", rep.Error)
	}
	return rep, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Call(ctx, CmdPing, nil)
	return err
}

func UnmarshalPayload[T any](rep Reply) (T, error) {
	var v T
	if len(rep.Payload) == 0 {
		return v, nil
	}
	err := json.Unmarshal(rep.Payload, &v)
	return v, err
}
