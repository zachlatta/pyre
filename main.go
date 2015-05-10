package main

import (
	"fmt"
	"flag"
	"os"
	"time"
	"sync"

	"github.com/jroimartin/gocui"
	"github.com/zachlatta/tinder"
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
	Recommendations []tinder.Recommendation
}

var (
	facebookUserID, facebookToken string
	tinderClient *tinder.Tinder
	profile TinderProfile
	profileMut sync.Mutex
	updateGUIChan chan struct{}

	// gui
	g *gocui.Gui
	backgroundView *gocui.View
	profilePictureView *gocui.View
	swipeLeftView *gocui.View
	swipeRightView *gocui.View
	infoView *gocui.View
)

func swipeLeft(g *gocui.Gui, v *gocui.View) error {
	if err := tinderClient.Pass(profile.Recommendations[0].ID); err != nil {
		fmt.Println(err)
	}
	profile.Recommendations = profile.Recommendations[1:]
	updateGUIChan<-struct{}{}
	return nil
}

func swipeRight(g *gocui.Gui, v *gocui.View) error {
	_, err := tinderClient.Like(profile.Recommendations[0].ID)
	if err != nil {
		fmt.Println(err)
	}
	profile.Recommendations = profile.Recommendations[1:]
	updateGUIChan<-struct{}{}
	return nil
}

func layout(g *gocui.Gui) error {
	var err error
	maxX, maxY := g.Size()
	if backgroundView, err = g.SetView("background", 0, 0, maxX, maxY); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
	}
    if profilePictureView, err = g.SetView("profilePicture", 1, 1, maxX-1, maxY-5); err != nil {
        if err != gocui.ErrorUnkView {
            return err
        }
		profilePictureView.Wrap = true
		fmt.Fprintln(profilePictureView, "Loading...")
    }
	if swipeLeftView, err = g.SetView("swipeLeft", 6, maxY-4, maxX/2-5, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprintln(swipeLeftView, "X")
	}
	if swipeRightView, err = g.SetView("swipeRight", maxX-(maxX/2-5), maxY-4, maxX-6, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprintln(swipeRightView, "<3")
	}
	if infoView, err = g.SetView("info", maxX/2-4, maxY-4, maxX/2+4, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprintln(infoView, "i")
	}
    return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.Quit
}

func keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'h', gocui.ModNone, swipeLeft); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'l', gocui.ModNone, swipeRight); err != nil {
		return err
	}
	return nil
}

func runGUI() error {
	g = gocui.NewGui()
	if err := g.Init(); err != nil {
		return err
	}
	defer g.Close()
	g.SetLayout(layout)
	if err := keybindings(g); err != nil {
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

func pollTinder(updateGUI chan struct{}, client *tinder.Tinder, profile *TinderProfile) {
	for _ = range time.Tick(5 * time.Second) {
		profileMut.Lock()
		resp, err := client.GetUpdates()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error polling:", err)
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

		if len(profile.Recommendations) == 0 {
			recsResp, err := client.GetRecommendations()
			if err != nil {
				if err != tinder.RecsExhausted && err != tinder.RecsTimeout {
					fmt.Fprintln(os.Stderr, "Error polling:", err)
				}
			} else {
				profile.Recommendations = recsResp.Results
			}
		}

		profileMut.Unlock()

		updateGUI<-struct{}{}
	}
}

func updateGUI(updateGUI chan struct{}) {
	for _ = range updateGUI {
		profilePictureView.Clear()
		if len(profile.Recommendations) > 0 {
			topRec := profile.Recommendations[0]
			fmt.Fprintf(profilePictureView, `Name: %s
Distance (mi): %d
----------------
%s`, topRec.Name, topRec.DistanceInMiles, topRec.Bio)
		} else {
			fmt.Fprintln(profilePictureView, "There's no one new around you.")
		}
		g.Flush()
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

	updateGUIChan = make(chan struct{})

	go pollTinder(updateGUIChan, tinderClient, &profile)
	go updateGUI(updateGUIChan)

	if err := runGUI(); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
}
