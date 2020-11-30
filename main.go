package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/i-am-g2/blockChain/blockchain"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery"
)

const DiscoveryInterval = time.Hour
const DiscoveryServiceTag = "pubsub"

var posType bool
var nick string

func main() {
	typeFlag := flag.Bool("cashPos", false, "Type of POS Terminal")
	flag.Parse()
	room := "PosChain"

	posType = *typeFlag
	blockchain.Balance = make(map[int]int)

	ctx := context.Background()

	h, err := libp2p.New(ctx, libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(err)
	}
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}

	err = setupDiscovery(ctx, h)
	if err != nil {
		panic(err)
	}

	nick = defaultNick(h.ID())

	// set up Log
	logFile := fmt.Sprintf("Logs/%s.log", nick)
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error Opening Log File")
	}
	defer f.Close()
	log.SetOutput(f)

	cr, err := blockchain.JoinChatRoom(ctx, ps, h.ID(), room)
	UI := NewChatUI(cr)
	if err := UI.Run(); err != nil {
		fmt.Println(err)
	}
}

func humanID(id peer.ID) string {
	prettyID := id.Pretty()
	return prettyID[len(prettyID)-4:]

}

type discoverNotifee struct {
	h host.Host
}

func (n *discoverNotifee) HandlePeerFound(pi peer.AddrInfo) {
	err := n.h.Connect(context.Background(), pi)

	if err != nil {
		fmt.Printf("Error Connecting to Peer %s %s\n", pi.ID.Pretty(), err)
	}
}

func setupDiscovery(ctx context.Context, h host.Host) error {
	disc, err := discovery.NewMdnsService(ctx, h, DiscoveryInterval, DiscoveryServiceTag)

	if err != nil {
		return err
	}
	n := discoverNotifee{h: h}
	disc.RegisterNotifee(&n)
	return nil
}

func defaultNick(p peer.ID) string {
	return fmt.Sprintf("%s", shortID(p))
}
