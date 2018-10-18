package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/fasmide/capture-all-the-scripts/server"
	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
)

var (
	port = flag.Int("port", 22, "specify listen port")
)

var (
	activeConnView *gocui.View
	statsView      *gocui.View
	logView        *gocui.View
)

func main() {

	flag.Parse()

	listenPath := fmt.Sprintf("0.0.0.0:%d", *port)
	eventChan := make(chan string)

	server := server.SSH{Path: listenPath, Events: eventChan}
	go server.Listen()

	log.Printf("listening on %s", listenPath)

	gui(&server, eventChan)
}

func gui(server *server.SSH, events chan string) {

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.SetManagerFunc(layout)

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}
	started := time.Now()
	go func() {
		for {
			s := server.State()
			sort.Sort(byStarted(s.Connections))

			g.Update(func(g *gocui.Gui) error {

				activeConnView.Clear()
				activeBytes := 0
				for _, item := range s.Connections {
					color.New(color.FgGreen).Fprintf(activeConnView, "%11s: %7s: %s\n",
						time.Now().Sub(item.Started).Truncate(time.Second).String(),
						humanize.Bytes(uint64(item.Written())),
						item.Remote,
					)
					activeBytes += item.Written()
				}
				activeConnView.Title = fmt.Sprintf("(%d) Active connections", len(s.Connections))
				statsView.Clear()
				fmt.Fprintf(statsView, "Total conns: \t%d\nTotal bytes: \t%s\nUptime: \t\t%7s\n",
					s.TotalConnections,
					humanize.Bytes(uint64(s.BytesSent+activeBytes)),
					time.Now().Sub(started).Truncate(time.Second),
				)

				return nil
			})

			time.Sleep(time.Millisecond * 500)
		}
	}()

	go func() {
		eventSlice := make([]string, 0, 50)
		for {
			event := <-events
			eventSlice = append(eventSlice, event)
			if len(eventSlice) >= 30 {
				eventSlice = eventSlice[len(eventSlice)-29 : 30]
			}
			g.Update(func(g *gocui.Gui) error {
				logView.Clear()
				for _, s := range eventSlice {
					color.New(color.FgYellow).Fprintf(logView, "%s\n", s)
				}
				return nil
			})

		}
	}()

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	var err error
	if activeConnView, err = g.SetView("activeconnections", 0, 0, maxX/2-1, maxY/2-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		activeConnView.Wrap = true
		activeConnView.Title = "(%d) Active Connections"
	}

	if statsView, err = g.SetView("stats", maxX/2-1, 0, maxX-1, maxY/2-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		statsView.Title = "Stats"
	}

	if logView, err = g.SetView("log", 0, maxY/2-1, maxX-1, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		logView.Title = "Log"
		logView.Autoscroll = true
	}

	return nil
}

type byStarted []*server.Connection

func (s byStarted) Len() int           { return len(s) }
func (s byStarted) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byStarted) Less(i, j int) bool { return s[i].Started.Before(s[j].Started) }
