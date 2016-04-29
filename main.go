package main

import (
	"goconf.googlecode.com/hg"
	"rand"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"exec"
	"strings"
	"time"
)

const (
	modeTrain = iota
	modeBattle
)

var c *Client
var configFile = flag.String("conf", "", "The filename holds the configuration to use.")
var modestring = flag.String("mode", "train", "Choose between battle and train.")
var psid = flag.String("psid", "", "A logged in PHPSESSID. Ninbot will use it instead of logging in.")
var noPopup = flag.Bool("no-popup", false, "Don't popup the captcha webpage. Instead, print it's URL.")

var cnfName, cnfPass string
var cnfRank int
var cnfActionSeq, cnfStatSeq []string
var cnfBattleRest, cnfTrainRest int

var mode int

func loadConf() (err os.Error) {
	cnf, err := conf.ReadConfigFile("conf\\" + *configFile)
	if err != nil {
		return
	}
	cnfName, err = cnf.GetString("account", "name")
	if err != nil {
		return
	}
	cnfPass, err = cnf.GetString("account", "password")
	if err != nil {
		return
	}
	rank, err := cnf.GetString("account", "rank")
	if err != nil {
		return
	}
	switch strings.ToLower(rank) {
	case "academy student":
		cnfRank = rankAcademyStudent
	case "genin":
		cnfRank = rankGenin
	case "chuunin":
		cnfRank = rankChuunin
	case "jounin":
		cnfRank = rankJounin
	case "special jounin":
		cnfRank = rankSpecialJounin
	default:
		return os.NewError(fmt.Sprintf("Invalid rank \"%s\"", rank))
	}
	actionSeq, err := cnf.GetString("battle", "sequence")
	if err != nil {
		return
	}
	cnfActionSeq = strings.Split(actionSeq, ",")
	for i, s := range cnfActionSeq {
		cnfActionSeq[i] = strings.TrimSpace(s)
	}
	statSeq, err := cnf.GetString("train", "sequence")
	if err != nil {
		return
	}
	cnfStatSeq = strings.Split(statSeq, ",")
	for i, s := range cnfStatSeq {
		cnfStatSeq[i] = strings.TrimSpace(s)
	}
	cnfBattleRest, err = cnf.GetInt("battle", "rest")
	cnfTrainRest, err = cnf.GetInt("train", "rest")
	return
}

func main() {
	flag.Parse()
	c = NewClient()

	err := loadConf()
	if err != nil {
		log.Fatalf("Could not load configuration file \"%s\": %s\n", *configFile, err)
	}
	log.Printf("Ninbot is running with configuration \"%s\"\n", *configFile)

	switch strings.ToLower(*modestring) {
	case "train":
		mode = modeTrain
	case "battle":
		mode = modeBattle
	default:
		log.Fatalf("Invalid mode \"%s\"\n", *modestring)
	}

	filename := time.LocalTime().Format("01.02.2006 15.04 05.000")
	f, err := os.OpenFile("log\\"+filename, os.O_WRONLY|os.O_CREATE, 666)
	if err != nil {
		log.Fatalln("Can't open the log file: ", err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.Ltime)

	if *psid != "" {
		c.PSID = *psid
		c.LoggedIn = true
	} else {
		log.Println("Ninbot is logging in")
		url, err := c.CaptchaURL(cnfName, cnfPass)
		if err != nil {
			log.Fatalln("Can't get captcha URL")
		}
		if *noPopup {
			log.Println("Open the following url, solve it and paste the resulting code:", url)
		} else {
			log.Println("A captcha window will popup, solve it and paste the resulting code")
			err = exec.Command("cmd", "/c", "start", url).Run()
			if err != nil {
				log.Fatalln("Can't open the captcha webpage with a browser:", err)
			}
		}
		var code string
		fmt.Scanln(&code)
		success, err := c.Login(code, cnfName, cnfPass)
		if err != nil {
			log.Fatalln("Can't login:", err)
		}
		if !success {
			log.Fatalln("Name, password or captcha proof code are wrong.")
		}
	}
	log.Printf("Logged in as %s with PHPSESSID = %s\n", cnfName, c.PSID)

	switch mode {
	case modeTrain:
		var nstat int
		for {
			stat := cnfStatSeq[nstat]
			res, err := c.Train(cnfRank, stat[1:], (stat[0] == '+'), -1)
			if err != nil {
				log.Fatalln("Can't train:", err)
			}
			log.Printf("Training improved %s by %f, now resting...\n", stat, res.GainStat)
			time.Sleep(int64(cnfTrainRest)*1e9 + rand.Int63n(2e9))

			nstat++
			if nstat == len(cnfStatSeq) {
				nstat = 0
			}
		}

	case modeBattle:
		var battles int
		for {
			log.Println("Entering battle...")
			opponent, err := c.EnterBattle()
			if err != nil {
				log.Fatalln("Failed to enter battle:", err)
			}
			log.Printf("Fighting %s\n", opponent)
			bg, err := c.Battleground()
			if err != nil {
				log.Fatalln("Failed to get battelground:", err)
			}
			var action int
			for {
				round, err := bg.Attack(c, cnfActionSeq[action], opponent)
				if err != nil {
					if err == ErrBattleFinished {
						break
					}
					log.Fatalln("Failed to attack:", err)
				}
				var s string
				for _, hit := range round.Hits {
					if hit.By == opponent {
						s += fmt.Sprintf("\tHe hits %d\t", int(hit.Damage))
					} else {
						s += fmt.Sprintf("\tYou hit %d\t", int(hit.Damage))
					}
				}
				log.Println(s)

				action++
				if action == len(cnfActionSeq) {
					action = 0
				}
			}
			battles++
			log.Printf("Battle number %d done\n", battles)
			success, err := c.EatAll()
			if err != nil {
				log.Fatalln("Failed to eat all:", err)
			}
			if success {
				log.Println("Ate all you can")
			} else {
				log.Println("Can't eat anymore")
			}
			log.Println("Resting a while...")
			time.Sleep(int64(cnfBattleRest)*1e9 + rand.Int63n(2e9))
		}
	}
}
