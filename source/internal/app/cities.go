package app

import "strings"

func knownUSCity(s string) bool {
	_, ok := usCities[normCity(s)]
	return ok
}

func normCity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ".", "")
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, "saint ", "st ")
	return s
}

func init() {
	usCities = make(map[string]struct{}, 512)
	for _, line := range strings.Split(usCityText, "\n") {
		line = normCity(line)
		if line == "" {
			continue
		}
		usCities[line] = struct{}{}
	}
}

var usCities map[string]struct{}

// Largest US places plus state capitals. Unknown-but-plausible names can still
// be kept after a confirm prompt.
const usCityText = `
akron
albuquerque
alexandria
allentown
amarillo
anaheim
anchorage
ann arbor
annapolis
arlington
atlanta
augusta
aurora
austin
bakersfield
baltimore
baton rouge
bellevue
billings
birmingham
boise
boston
boulder
bridgeport
buffalo
burbank
cambridge
cape coral
carson city
cary
charleston
charlotte
chattanooga
chesapeake
chicago
chula vista
cincinnati
clarksville
cleveland
colorado springs
columbia
columbus
concord
coral springs
corona
corpus christi
dallas
dayton
denver
des moines
detroit
durham
el paso
elk grove
eugene
evansville
fairfield
fargo
fayetteville
fontana
fort collins
fort lauderdale
fort wayne
fort worth
fremont
fresno
frisco
fullerton
gainesville
garden grove
garland
gilbert
glendale
grand prairie
grand rapids
greensboro
henderson
hialeah
hollywood
honolulu
houston
huntington beach
huntsville
indianapolis
irvine
irving
jackson
jacksonville
jersey city
joliet
kansas city
knoxville
lafayette
lakeland
lansing
laredo
las cruces
las vegas
lexington
lincoln
little rock
long beach
los angeles
louisville
lubbock
madison
manchester
mcallen
mckinney
memphis
mesa
miami
midland
milwaukee
minneapolis
mobile
modesto
montgomery
moreno valley
murfreesboro
nashville
new haven
new orleans
new york
new york city
newark
newport news
norfolk
north las vegas
oakland
oklahoma city
olympia
omaha
ontario
orlando
overland park
oxnard
palmdale
paradise
pasadena
paterson
pembroke pines
peoria
philadelphia
phoenix
pittsburgh
plano
portland
providence
provo
pueblo
raleigh
rancho cucamonga
reno
richmond
riverside
rochester
rockford
sacramento
salem
salinas
salt lake city
san antonio
san bernardino
san diego
san francisco
san jose
santa ana
santa clara
santa clarita
santa rosa
savannah
scottsdale
seattle
shreveport
sioux falls
spokane
springfield
st louis
st paul
st petersburg
stamford
sterling heights
stockton
sunnyvale
syracuse
tacoma
tallahassee
tampa
tempe
thornton
toledo
topeka
torrance
tucson
tulsa
vancouver
virginia beach
visalia
waco
warren
washington
washington dc
west valley city
wichita
wichita falls
wilmington
winston salem
worcester
yonkers
`
