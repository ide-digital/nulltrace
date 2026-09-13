package exposure

var commonPasswords = map[string]struct{}{}

func init() {
	for _, w := range stringsSplit(commonPasswordBlob) {
		commonPasswords[w] = struct{}{}
	}
}

// Frequent passwords from public "most used" rankings. Used only for a
// local check — the value is never sent to the network from this list.
const commonPasswordBlob = `
password 123456 123456789 12345678 qwerty abc123 111111 1234567 monkey
password1 123123 000000 iloveyou 1234 1q2w3e4r qwertyuiop 123 qwerty123
admin letmein welcome dragon master login princess football shadow
passw0rd basebal sunshine 654321 superman qazwsx michael football1
password123 1234567890 starwars hello freedom whatever qwerty1 charlie
aa123456 donald password12 pass123 soccer harley hunter ranger cookie
trustno1 hunter2 secret 12345 123456a photoshop 696969 555555 7777777
121212 flower andrew tigger robert daniel thomas hockey killer maggie
jessica pepper zdnk batman summer iloveu ashley 654321a computer
michelle ashley1 pepper1 jordan 123qwe abc1234 1qaz2wsx zaq12wsx
password2 passwort passwort1 passwort123 kennwort welcome1 welcome123
p@ssw0d p@ssword p@ssw0rd changeme default root toor admin123 admin1
letmein1 dragon1 monkey1 mustang michael1 shadow1 jennifer jordan23
superman1 00000000 11111111 aaaaaa abcdef abcdefg abcdefgh asdfgh
asdfghjkl zxcvbn zxcvbnm qweasd qweasdzxc 1q2w3e 1q2w3e4r5t
q1w2e3r4 123321 654321 112233 102030 123abc abc12345 pass pass1
pass12 pass1234 pass12345 0000 1111 2222 3333 4444 5555 6666 7777
8888 9999 1212 1313 6969 7777 aaaa aaaa1 aaaaa aaa123 love loveme
iloveyou1 iloveyou2 princess1 sunshine1 chocolate whatever1
hello123 hello1 welcome2 football12 baseball baseball1 basketball
hockey1 soccer1 ranger1 hunter12 cookie1 pepper12 tigger1 robert1
daniel1 thomas1 killer1 maggie1 jessica1 summer1 ashley12 computer1
michelle1 ncc1701 starwars1 freedom1 whatever12 qwerty12 charlie1
donald1 photoshop1 1qazxsw2 zaq1xsw2 !qaz2wsx p@ssw0rd1 Password
Password1 Password123 P@ssw0rd P@ssword1 Winter2024 Winter2025
Spring2024 Summer2024 Fall2024 January February Monday Tuesday
Welcome1 Welcome123 Admin123 Root123 Letmein1 Dragon123 Monkey123
Qwerty1 Qwerty123 Abc123456 123456Aa Aa123456 password! password@
passw0rd1 passw0rd123 123456789a 12345678a qwertyui 1q2w3e4r5t6y
qwerty1234 asdf1234 zxcv1234 pokemon naruto fortnite minecraft
roblox twitch discord steam xbox playstation nintendo google
facebook instagram twitter youtube netflix amazon apple microsoft
samsung iphone android gmail yahoo hotmail outlook
`
