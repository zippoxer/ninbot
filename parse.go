package main

import (
	"regexp"
	"strconv"
	"strings"
	"os"
)

const (
	statusAwake = iota
	statusBattle
	statusAsleep
)

const (
	rankAcademyStudent = iota
	rankGenin
	rankChuunin
	rankJounin
	rankSpecialJounin
)

type Sidebar struct {
	InBattle, Hospitalized bool
	LogoutTimer            float32
}

type HealthChakraStamina struct {
	Health, MaxHealth   float32
	Chakra, MaxChakra   float32
	Stamina, MaxStamina float32
}

type LoginCaptchaPage struct {
	CaptchaURL string
}

type ProfilePage struct {
	Sidebar
	HealthChakraStamina

	Level                                       int
	Rank                                        string
	Money, BankedMoney                          int
	ANBU                                        bool
	Clan                                        string
	Experience, NeededExperience, PVPExperience int

	Status     int
	RegenRate  float32
	RegenTimer int
}

type ProfileWithStatsPage struct {
	ProfilePage
	Name, Gender, Email             string
	TaiStr, NinStr, GenStr, WeapStr float32
	TaiDef, NinDef, GenDef, WeapDef float32
	Str, Int, Speed, Will           float32
}

type BattleEntrancePage struct {
	Sidebar
	LeftLink, RightLink, LeftImage, RightImage string
}

type BattlePreparePage struct {
	Sidebar
	OpponentName string
}

type BattlegroundPage struct {
	Sidebar
	HealthChakraStamina
	ID                  int
	Actions             map[string]string
	Opponents           map[string]int
	YourActionSubmitted bool // whether the page contains "Your action has been submitted" message or not.
}

type BattleHit struct {
	Damage float32
	By, To string
}

type BattleRoundPage struct {
	Sidebar
	Hits []BattleHit
}

type TrainAmountSelectionPage struct {
	Sidebar
	MaxAmount int
}

type TrainResultPage struct {
	Sidebar
	GainExp                   int
	GainStat, GainChakra      float32
	SpentChakra, SpentStamina float32
}

var regexpLogoutTimer = regexp.MustCompile(`<b>Logout timer:</b>.+<noscript>([0-9]+) minutes (([0-9]+) seconds)*`)

func ParseSidebar(input string) (b Sidebar, err os.Error) {
	if strings.Contains(input, "<noscript>1 hour") {
		b.LogoutTimer = 60
	} else {
		matches := regexpLogoutTimer.FindStringSubmatch(input)
		if len(matches) != 4 {
			return b, os.NewError("Failed to parse the logout timer in the sidebar")
		}
		mins, _ := strconv.Atof32(matches[1])
		secs, _ := strconv.Atof32(matches[3])
		b.LogoutTimer = mins + (secs / 60)
	}
	b.InBattle = strings.Contains(input, `<a href="?id=41">In battle!</a>`)
	b.Hospitalized = strings.Contains(input, `<a href="?id=34">Hospitalized!</a>`)
	return
}

var regexpCaptchaURL = regexp.MustCompile("<iframe src=\"([^\"]+)")

func ParseLoginCaptchaPage(input string) (page LoginCaptchaPage, err os.Error) {
	matches := regexpCaptchaURL.FindStringSubmatch(input)
	if len(matches) != 2 {
		return
	}
	page.CaptchaURL = matches[1]
	return
}

var regexpBattleEntrance = regexp.MustCompile(`<a href="(\?id=35&act=[^"]+)"><img src=\.(/images/antibot/[^>]+)></a> <img src=\./images/antibot/or\.gif> <a href="(\?id=35&act=[^"]+)"><img src=\.(/images/antibot/[^>]+)></a>`)

func ParseBattleEntrancePage(input string) (page BattleEntrancePage, err os.Error) {
	page.Sidebar, err = ParseSidebar(input)
	if err != nil {
		return
	}
	matches := regexpBattleEntrance.FindStringSubmatch(input)
	if len(matches) != 5 {
		return
	}
	page.LeftLink = "/" + matches[1]
	page.RightLink = "/" + matches[3]
	page.LeftImage = matches[2]
	page.RightImage = matches[4]
	return
}

var regexpOpponentName = regexp.MustCompile(`<td align="center" style="font-weight:bold;">([^<]+)</td>`)

func ParseBattlePreparePage(input string) (page BattlePreparePage, err os.Error) {
	page.Sidebar, err = ParseSidebar(input)
	if err != nil {
		return
	}
	matches := regexpOpponentName.FindStringSubmatch(input)
	if len(matches) != 2 {
		return page, os.NewError("Failed to parse battle prepare page")
	}
	page.OpponentName = matches[1]
	return
}

var regexpBattleID = regexp.MustCompile(`<input type="hidden" name="battle_id" value="([0-9]+)">`)
var regexpAction = regexp.MustCompile(`<input name="action" type="radio" value="([^"]+)" (Checked)?> ([^<]+)`)
var regexpOpponent = regexp.MustCompile(`input name="opponent" type="radio" value="([0-9]+)" (Checked)?> ([^<]+)`)

func ParseBattlegroundPage(input string) (page BattlegroundPage, err os.Error) {
	page.Sidebar, err = ParseSidebar(input)
	if err != nil {
		return
	}

	if strings.Contains(input, `<td align="center">Your action has been submitted`) {
		page.YourActionSubmitted = true
		return
	}

	matches := regexpBattleID.FindStringSubmatch(input)
	if len(matches) != 2 {
		err = os.NewError("Failed to parse battleground page: couldn't parse battle ID")
		return
	}
	page.ID, _ = strconv.Atoi(matches[1])

	actionMatches := regexpAction.FindAllStringSubmatch(input, -1)
	if len(actionMatches) == 0 {
		err = os.NewError("Failed to parse battleground page: no actions found")
		return
	}
	page.Actions = make(map[string]string)
	for _, match := range actionMatches {
		name := strings.ToLower(strings.TrimSpace(match[3]))
		page.Actions[name] = match[1]
	}

	opponentMatches := regexpOpponent.FindAllStringSubmatch(input, -1)
	if len(opponentMatches) == 0 {
		err = os.NewError("Failed to parse battleground page: no opponents found")
		return
	}
	page.Opponents = make(map[string]int)
	for _, match := range opponentMatches {
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return page, err
		}
		name := strings.ToLower(strings.TrimSpace(match[3]))
		page.Opponents[name] = n
	}
	return
}

var regexpDeal = regexp.MustCompile(`<font color="#000080"><i>([^<]+)</i> deals ([0-9\.]+) [a-z]* *damage to <i>([^<]+)</i></font>`)

func ParseBattleRoundPage(input string) (page BattleRoundPage, err os.Error) {
	page.Sidebar, err = ParseSidebar(input)
	if err != nil {
		return
	}
	if !strings.Contains(input, `<td align="center" style="border-top:none;" class="subHeader">Outcome:</td>`) {
		err = os.NewError("Failed to parse battle round page")
		return
	}
	matches := regexpDeal.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		err = os.NewError("Failed to parse battle round page: no damage deals were found")
		return
	}
	for _, match := range matches {
		var hit BattleHit
		hit.Damage, err = strconv.Atof32(match[2])
		if err != nil {
			return
		}
		hit.By, hit.To = match[1], match[3]
		page.Hits = append(page.Hits, hit)
	}
	return
}

func IsBattleSummaryPage(input string) bool {
	return strings.Contains(input, `>Battle summary:</td>`)
}

func IsMaintenancePage(input string) bool {
	return strings.Contains(input, `Maintenance`)
}

var regexpMaxAmount = regexp.MustCompile(`>([0-9]+)</option></select>`)

func ParseTrainAmountSelectionPage(input string) (page TrainAmountSelectionPage, err os.Error) {
	page.Sidebar, err = ParseSidebar(input)
	if err != nil {
		return
	}
	matches := regexpMaxAmount.FindStringSubmatch(input)
	if len(matches) != 2 {
		err = os.NewError("Failed to parse train amount selection page")
		return
	}
	page.MaxAmount, _ = strconv.Atoi(matches[1])
	return
}

var regexpTrainResult = regexp.MustCompile(`You gained ([0-9]+) exp.+You improved ([0-9\.]+) points in`)

func ParseTrainResultPage(input string) (page TrainResultPage, err os.Error) {
	page.Sidebar, err = ParseSidebar(input)
	if err != nil {
		return
	}
	matches := regexpTrainResult.FindStringSubmatch(input)
	if len(matches) != 3 {
		err = os.NewError("Failed to parse train result page")
		return
	}
	exp, _ := strconv.Atoi(matches[1])
	stat, err := strconv.Atof32(matches[2])
	if err != nil {
		return
	}
	page.GainExp = int(exp)
	page.GainStat = float32(stat)
	return
}
