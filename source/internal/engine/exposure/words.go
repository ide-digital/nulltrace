package exposure

// Compact word list for local passphrase suggestions. Five or more
// words picked with crypto/rand is enough entropy for this screen.
var wordList = stringsSplit(`
able acid acre aged aid aim air ale all ally almond amber amen analog
anchor anvil apple april arbor area arena armor army aroma arrow ash
atlas atom attic audio aunt aura aurora avail award aware axis bacon
badge bagel baker balm bamboo banana band barge barn basil basin batch
baton beach bead beam bean bear beaver beech beer beet begin beige
belly bench berry bike bind birch bird bison bite black blade blank
blaze blend blind bliss block bloom blouse blue blunt board boast boat
body boil bold bolt bone book boost boot bore born boss bottle bound
bow bowl box boy braid brain brake brand brass brave bread break brew
brick bride brief brim bring brisk broad broth brown brush bubble buck
buddy budget buffer build bulb bulk bully bunch bunny burden bureau
burn burst bus bush bust busy butter button buyer buzz cabin cable
cactus cage cake calf call calm camel camp canal candy canoe canvas
canyon capable cape car carbon card cargo carol carp carry cart case
cash casino castle catch cat cattle caught cause cave cedar ceiling
cell cement census century cereal chain chair chalk champion chance
change chaos chapel charge charm chart chase cheap check cheese chef
cherry chest chick chief child chili chill china chip chocolate choice
choir chop chord chorus chrome cider cinema circle citrus city civic
civil claim clamp clams clan clap class claw clay clean clear clerk
click client cliff climate climb clip clock clone close cloth cloud
clove club clue coach coast coat cobra cocoa code coffee coil coin
coke cold collar college color column combo comet comic comma common
coral cord core cork corn corner corpus correct cost cotton couch
cough could count county couple course court cover coyote crack craft
crane crash crate crayon crazy cream credit creek crew crib cricket
crime crisp critic crop cross crow crowd crown crude crush crust cub
cube cucumber cuff culture cup cupboard cure curl curry curtain curve
cushion custom cute cycle dad daily dairy daisy dam dance danger dare
dark dart dash data date daughter dawn day deal dean dear death debit
decade decimal deck deer delay delta deluxe demand dense dent deny
depot depth desk destiny device devil diamond diary dice diet dig
digit dill dime diner dinghy dinner diode dirt discover dish disk
ditch dive dock dodge doll dolphin domain donate donkey donor door
dose double dove down dozen draft dragon drain drama draw dream dress
drift drill drink drip drive drop drum dry dual duck duct due duffel
dug dune dunk dusk dust duty dwarf eagle early earn earth easel east
easy eat echo edge eel egg eight either elbow elder electric elegant
element elephant elevator elite else ember emerald emit empire empty
emu enable enamel end energy engine enjoy enlist enough entry envoy
envy equal era erase error erupt essay essence estate etch eternal
ethics ethnic even event every evil evoke evolve exact exam exceed
excel except excess excite excuse exercise exhaust exhibit exile exist
exit exotic expand expect expire explain expose extend extra eye fabric
face fact fade faint fair fake fall false fame family famous fan fancy
fang far farm fashion fast fat fatal father fatty faucet fault fauna
favor feast feather feature february federal fee feeble feed feel
female fence fend fern ferry fetal fetch fever few fiber fiction field
fifteen fifth fifty fight figure file fill film filter final finance
find fine finger finish fire firm first fish fist fit five fix flag
flair flake flame flap flash flat flavor flax flee flight flip float
flock flood floor flour flow flower fluent fluid flush flux fly foam
focus fog foil fold folk follow food fool foot force forest forget
fork form fort fortune forum fossil foster found fox foyer fraction
fragile frame frank fraud fray freak free freeze french frenzy fresh
friend fringe frog front frost frown frozen fruit fry fuel full fume
fun fund funny furnace fur fury fuse fuss future fuzzy
`)

func stringsSplit(s string) []string {
	var out []string
	cur := make([]byte, 0, 12)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		out = append(out, string(cur))
		cur = cur[:0]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			flush()
			continue
		}
		cur = append(cur, c)
	}
	flush()
	return out
}
