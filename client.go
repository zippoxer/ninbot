package main

import (
	"bytes"
	"io/ioutil"
	"http"
	"strconv"
	"url"
	"strings"
	"time"
	"os"
	"fmt"
)

var (
	ErrNotLoggedIn      = os.NewError("Not logged in")
	ErrNotInBattle      = os.NewError("Not in battle")
	ErrNotAwake         = os.NewError("Not awake")
	ErrBattleFinished   = os.NewError("Battle is finished")
)

type Battleground BattlegroundPage
type BattleRound BattleRoundPage
type TrainResult TrainResultPage

type Client struct {
	hc       *http.Client
	PSID     string // the PHPSESSID generated by logging in
	LoggedIn bool
	Status   int
}

func NewClient() *Client {
	return &Client{
		hc:   &http.Client{},
	}
}

func (c *Client) Do(method string, path string, values url.Values) (*http.Response, os.Error) {
	url := "http://www.theninja-rpg.com" + path
	req, err := http.NewRequest(method, url, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; rv:8.0.1) Gecko/20100101 Firefox/8.0.1")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.PSID != "" {
		req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: c.PSID})
	}
	var tries int
try:
	resp, err := c.hc.Do(req)
	if err != nil {
		tries++
		if tries == 5 {
			return nil, err
		}
		goto try
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "PHPSESSID" {
			c.PSID = cookie.Value
		}
	}
	return resp, nil
}

func (c *Client) Get(path string) (*http.Response, os.Error) {
	return c.Do("GET", path, nil)
}

func (c *Client) Post(path string, values url.Values) (*http.Response, os.Error) {
	return c.Do("POST", path, values)
}

func (c *Client) ReadGet(path string) (*http.Response, string, os.Error) {
	resp, err := c.Do("GET", path, nil)
	if err != nil {
		return nil, "", err
	}
	data, err := c.Read(resp)
	return resp, data, err
}

func (c *Client) ReadPost(path string, values url.Values) (*http.Response, string, os.Error) {
	resp, err := c.Do("POST", path, values)
	if err != nil {
		return nil, "", err
	}
	data, err := c.Read(resp)
	return resp, data, err
}

func (c *Client) Read(resp *http.Response) (string, os.Error) {
	var tries int
try:
	data, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		if IsMaintenancePage(string(data)) {
			time.Sleep(10e9)
			goto try
		}
		tries++
		if tries == 5 {
			return "", err
		}
		goto try
	}
	return string(data), nil
}

func (c *Client) CaptchaURL(name, pass string) (string, os.Error) {
	_, data, err := c.ReadPost("/?id=1", url.Values{
		"lgn_usr_stpd":   {name},
		"login_password": {pass},
		"LoginSubmit":    {"Submit"},
	})
	if err != nil {
		return "", err
	}
	page, err := ParseLoginCaptchaPage(data)
	if err != nil {
		return "", err
	}
	return page.CaptchaURL, nil
}

func (c *Client) Login(proofCode, name, pass string) (bool, os.Error) {
	resp, err := c.Post("/?id=1", url.Values{
		"recaptcha_challenge_field": {proofCode},
		"recaptcha_response_field":  {"manual_challenge"},
		"lgn_usr_stpd":              {name},
		"login_password":            {pass},
		"LoginSubmit":               {"Submit"},
	})
	if err != nil {
		return false, err
	}
	c.LoggedIn = resp.Header.Get("Location") == "?id=1"
	return c.LoggedIn, nil
}

func (c *Client) detectEnterBattleLink(page BattleEntrancePage) (string, os.Error) {
	leftResp, err := c.Get(page.LeftImage)
	if err != nil {
		return "", err
	}
	rightResp, err := c.Get(page.RightImage)
	if err != nil {
		return "", err
	}
	leftSize := leftResp.Header.Get("Content-Length")
	rightSize := rightResp.Header.Get("Content-Length")
	if leftSize > rightSize {
		return page.LeftLink, nil
	}
	return page.RightLink, nil
}

func (c *Client) EnterBattle() (opponentName string, err os.Error) {
	if err = c.require(statusAwake); err != nil {
		return
	}
	_, data, err := c.ReadGet("/?id=35")
	if err != nil {
		return
	}
	page, err := ParseBattleEntrancePage(data)
	if err != nil {
		return
	}
	link, err := c.detectEnterBattleLink(page)
	if err != nil {
		return
	}
	_, data, err = c.ReadGet(link)
	if err != nil {
		return
	}
	preparePage, err := ParseBattlePreparePage(data)
	if err != nil {
		return
	}
	opponentName = preparePage.OpponentName
	c.Status = statusBattle
	return
}

func (c *Client) Battleground() (bg Battleground, err os.Error) {
	if err = c.require(statusBattle); err != nil {
		return
	}
	_, data, err := c.ReadGet("/?id=41")
	if err != nil {
		return
	}
	page, err := ParseBattlegroundPage(data)
	if err != nil {
		return
	}
	bg = Battleground(page)
	return
}

func (c *Client) Attack(battleID int, actionID string, opponentID int) (round BattleRound, err os.Error) {
	if err = c.require(statusBattle); err != nil {
		return
	}
	_, data, err := c.ReadPost("/?id=41&act=do", url.Values{
		"action":    {actionID},
		"opponent":  {strconv.Itoa(opponentID)},
		"Submit":    {"Submit"},
		"battle_id": {strconv.Itoa(battleID)},
	})
	if err != nil {
		return
	}
	if IsBattleSummaryPage(data) {
		c.Status = statusAwake
		err = ErrBattleFinished
		return
	}
	page, err := ParseBattlegroundPage(data)
	if err != nil {
		return
	}
	if !page.YourActionSubmitted {
		err = os.NewError("No \"Your action has been submitted\" after attacking")
		return
	}
	_, data, err = c.ReadGet("/?id=41")
	if err != nil {
		return
	}
	roundPage, err := ParseBattleRoundPage(data)
	return BattleRound(roundPage), err
}

func (b *Battleground) Attack(c *Client, action, opponent string) (BattleRound, os.Error) {
	actionID, ok := b.Actions[strings.ToLower(action)]
	if !ok {
		return BattleRound{}, os.NewError("Action does not exist")
	}
	opponentID, ok := b.Opponents[strings.ToLower(opponent)]
	if !ok {
		return BattleRound{}, os.NewError("Opponent does not exist")
	}
	return c.Attack(b.ID, actionID, opponentID)
}

func (c *Client) EatAll() (success bool, err os.Error) {
	if err = c.require(statusAwake); err != nil {
		return
	}
	_, data, err := c.ReadGet("/?id=25&buy=8")
	if err != nil {
		return
	}
	if _, err = ParseSidebar(data); err != nil {
		return
	}
	if strings.Contains(data, `No more food for you`) {
		return
	}
	if strings.Contains(data, `You pay for your dinner and quietly enjoy it`) {
		success = true
		return
	}
	err = os.NewError("Failed to eat: unexpected response after ordering \"eat all you can\"")
	return
}

func (c *Client) Train(rank int, what string, offensive bool, amount int) (res TrainResult, err os.Error) {
	if err = c.require(statusAwake); err != nil {
		return
	}
	var pageId int
	switch rank {
	case rankAcademyStudent:
		pageId = 18
	case rankGenin:
		pageId = 29
	case rankChuunin:
		pageId = 39
	default:
		err = os.NewError("Training is not supported for rank")
		return
	}
	pageUrl := fmt.Sprintf("/?id=%d&page=train", pageId)
	offensivestring := "Offensive"
	if !offensive {
		offensivestring = "Defensive"
	}
	if amount == -1 {
		_, data, err := c.ReadPost(pageUrl, url.Values{
			"train":    {what},
			"do_train": {offensivestring},
			"Submit":   {"Train"},
		})
		if err != nil {
			return res, err
		}
		page, err := ParseTrainAmountSelectionPage(data)
		if err != nil {
			return res, err
		}
		amount = page.MaxAmount
	}
	_, data, err := c.ReadPost(pageUrl, url.Values{
		"train_amount": {strconv.Itoa(amount)},
		"train":        {what},
		"do_train":     {offensivestring},
		"Submit":       {"Train"},
	})
	if err != nil {
		return
	}
	page, err := ParseTrainResultPage(data)
	if err != nil {
		return
	}
	return TrainResult(page), nil
}

func (c *Client) require(status int) os.Error {
	if !c.LoggedIn {
		return ErrNotLoggedIn
	}
	if status == statusAwake && c.Status != statusAwake {
		return ErrNotAwake
	}
	if status == statusBattle && c.Status != statusBattle {
		return ErrNotInBattle
	}
	/*if status == statusAsleep && c.Status != statusAsleep {
		return ErrNotAsleep
	}*/
	return nil
}

//func (c *Client) RunErrands(amount int) (res ErrandsResult, err os.Error)
//func (c *Client) Sleep() (bool, err)
//func (c *Client) Wake() (bool, err)