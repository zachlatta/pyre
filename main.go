package main

import (
	"fmt"
	"flag"
	"os"
	"time"
	"sync"

	"github.com/jroimartin/gocui"
	"github.com/norwack/tinder"
)

type Message struct {
	ID string
	MatchID string
	Timestamp int64
	To string
	From string
	Message string
	Sent string
}

type Person struct {
	ID string
	Bio string
	Birth string
	Gender int
	Name string
	PingTime string
}

type Match struct {
	ID string
	CommonFriendCount int
	CommonLikeCount int
	MessageCount int
	Messages []*Message
	Person *Person
}

type TinderProfile struct {
	Matches []*Match
}

var (
	facebookUserID, facebookToken string
	tinderClient *tinder.Tinder
	profile TinderProfile
	profileMut sync.Mutex
)

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if _, err := g.SetView("background", 0, 0, maxX, maxY); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
	}
    if profilePicture, err := g.SetView("profilePicture", 1, 1, maxX-1, maxY-5); err != nil {
        if err != gocui.ErrorUnkView {
            return err
        }
        fmt.Fprintln(profilePicture, "example")
    }
	if swipeLeft, err := g.SetView("swipeLeft", 6, maxY-4, maxX/2-5, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprintln(swipeLeft, "X")
	}
	if swipeRight, err := g.SetView("swipeRight", maxX-(maxX/2-5), maxY-4, maxX-6, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprintln(swipeRight, "<3")
	}
	if info, err := g.SetView("info", maxX/2-4, maxY-4, maxX/2+4, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprintln(info, "i")
	}
    return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.Quit
}

func runGUI() error {
	g := gocui.NewGui()
	if err := g.Init(); err != nil {
		return err
	}
	defer g.Close()
	g.SetLayout(layout)
	if err := g.SetKeybinding(
		"", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	g.SelBgColor = gocui.ColorGreen
	g.SelFgColor = gocui.ColorBlack
	err := g.MainLoop()
	if err != nil && err != gocui.Quit {
		return err
	}
	return nil
}

func pollTinder(client *tinder.Tinder, profile *TinderProfile) {
	for _ = range time.Tick(5 * time.Second) {
		profileMut.Lock()
		resp, err := client.GetUpdates()
		if err != nil {
			fmt.Println("Error polling:", err)
		}
		matches := make([]*Match, len(resp.Matches))
		for i, match := range resp.Matches {
			matches[i] = &Match{
				ID: match.ID,
				CommonFriendCount: match.CommonFriendCount,
				CommonLikeCount: match.CommonLikeCount,
				MessageCount: match.MessageCount,
				Person: &Person{
					ID: match.Person.ID,
					Bio: match.Person.Bio,
					Birth: match.Person.Birth,
					Gender: match.Person.Gender,
					Name: match.Person.Name,
					PingTime: match.Person.PingTime,
				},
			}
			messages := make([]*Message, len(match.Messages))
			for i, msg := range match.Messages {
				messages[i] = &Message{
					ID: msg.ID,
					MatchID: msg.MatchID,
					Timestamp: msg.Timestamp,
					To: msg.To,
					From: msg.From,
					Message: msg.Message,
					Sent: msg.Sent,
				}
			}
			matches[i].Messages = messages
		}
		profile.Matches = matches
		profileMut.Unlock()
	}
}

func main() {
	flag.StringVar(&facebookUserID, "fb_user_id", "", "facebook user id")
	flag.StringVar(&facebookToken, "fb_token", "", "facebook token")
	flag.Parse()

	if flag.NArg() == flag.NFlag() {
		fmt.Fprintf(os.Stderr, "Error: all flags are required. Run with -help for options.\n")
		os.Exit(1)
	}

	tinderClient = tinder.Init(facebookUserID, facebookToken)
	if err := tinderClient.Auth(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not authenticate: ", err.Error())
		os.Exit(1)
	}

	go pollTinder(tinderClient, &profile)

	if err := runGUI(); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
}
