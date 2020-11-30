package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/i-am-g2/blockChain/blockchain"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rivo/tview"
)

// ChatUI A
type ChatUI struct {
	cr       *blockchain.ChatRoom
	app      *tview.Application
	peerList *tview.TextView

	msgW    io.Writer
	inputCh chan *blockchain.Transaction
	doneCh  chan struct{}
}

func getPosType(posType bool) string {
	if posType {
		return "Cash Terminal"
	} else {
		return "Retail Terminal"
	}
}

func NewChatUI(cr *blockchain.ChatRoom) *ChatUI {
	app := tview.NewApplication()
	msgBox := tview.NewTextView()
	msgBox.SetDynamicColors(true)
	msgBox.SetTitle(fmt.Sprintf("Message Logs - %s | %v", nick, getPosType(posType)))
	msgBox.SetBorder(true)
	msgBox.SetScrollable(true)

	msgBox.SetChangedFunc(func() {
		app.Draw()
	})

	inputCh := make(chan *blockchain.Transaction, 32)
	input := tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)

	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		line := input.GetText()
		if len(line) == 0 {
			return
		}

		if line == "/quit" {
			app.Stop()
			return
		}

		currentTransaction, err := handleInput(line, msgBox)
		if err != nil {
			input.SetText("")
			return
		}
		if blockchain.Balance[currentTransaction.CardID]+currentTransaction.Amount < 0 {
			fmt.Fprintf(msgBox, "%s Insufficient Balance\n", withColor("blue", "<System>:"))
			input.SetText("")
			return
		}
		currentTransaction.SignerID = nick
		inputCh <- currentTransaction
		input.SetText("")
	})

	peersList := tview.NewTextView()
	peersList.SetBorder(true)
	peersList.SetTitle("Peers")

	chatPanel := tview.NewFlex().
		AddItem(msgBox, 0, 1, false).
		AddItem(peersList, 20, 1, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(chatPanel, 0, 1, false).
		AddItem(input, 1, 1, true)

	app.SetRoot(flex, true)

	return &ChatUI{
		cr:       cr,
		app:      app,
		peerList: peersList,
		msgW:     msgBox,
		inputCh:  inputCh,
		doneCh:   make(chan struct{}, 1),
	}
}

// Run starts the chat event loop in the background, then starts
// the event loop for the text UI.
func (ui *ChatUI) Run() error {
	go ui.handleEvents()
	defer ui.end()

	return ui.app.Run()
}

// end signals the event loop to exit gracefully
func (ui *ChatUI) end() {
	ui.doneCh <- struct{}{}
}

func shortID(p peer.ID) string {
	idStr := p.Pretty()
	return idStr[len(idStr)-4:]
}

// refreshPeers pulls the list of peers currently in the chat room and
// displays the last 8 chars of their peer id in the Peers panel in the ui.
func (ui *ChatUI) refreshPeers() {
	peers := ui.cr.ListPeers()
	idStrs := make([]string, len(peers))
	for i, p := range peers {
		idStrs[i] = shortID(p)
	}

	ui.peerList.SetText(strings.Join(idStrs, "\n"))
	ui.app.Draw()
}

// displayChatMessage writes a ChatMessage from the room to the message window,
// with the sender's nick highlighted in green.
func (ui *ChatUI) displayChatMessage(cm blockchain.UIMessage) {
	prompt := withColor("green", fmt.Sprintf("<%s>:", cm.SenderID[len(cm.SenderID)-4:]))
	fmt.Fprintf(ui.msgW, "%s : %s\n", prompt, cm.Message)
}

// displaySelfMessage writes a message from ourself to the message window,
// with our nick highlighted in yellow.
func (ui *ChatUI) displaySelfMessage(msg string) {
	prompt := withColor("yellow", fmt.Sprintf("<%s>:", nick))
	fmt.Fprintf(ui.msgW, "%s %s\n", prompt, msg)
}

// handleEvents runs an event loop that sends user input to the chat room
// and displays messages received from the chat room. It also periodically
// refreshes the list of peers in the UI.
func (ui *ChatUI) handleEvents() {
	peerRefreshTicker := time.NewTicker(time.Second)
	defer peerRefreshTicker.Stop()

	for {
		select {
		case input := <-ui.inputCh:
			// Handle Input here
			// when the user types in a line, publish it to the chat room and print to the message window
			err := ui.cr.Publish(*input)
			if err != nil {
				// printErr("publish error: %s", err)
				fmt.Println(err)
			}
			ui.displaySelfMessage(input.AsString())
			// fmt.Println(input)
			// log.Println(input)
		case m := <-ui.cr.Messages:
			// when we receive a message from the chat room, print it to the message window
			ui.displayChatMessage(m)

		case <-peerRefreshTicker.C:
			// refresh the list of peers in the chat room periodically
			ui.refreshPeers()

		case <-ui.cr.Ctx.Done():
			return

		case <-ui.doneCh:
			return
		}
	}
}

// withColor wraps a string with color tags for display in the messages text box.
func withColor(color, msg string) string {
	return fmt.Sprintf("[%s]%s[-]", color, msg)
}

func handleInput(line string, msgB *tview.TextView) (*blockchain.Transaction, error) {
	splits := strings.Split(line, " ")
	if len(splits) != 2 {
		return nil, errors.New("Invalid Input")
	}

	if splits[0] == "/balance" {
		cardID, err := strconv.Atoi(splits[1])
		if err != nil {
			return nil, errors.New("Invalid Input")
		}
		showBalance(cardID, msgB)
		return nil, errors.New("Invalid Input")
	}

	if splits[0] == "/dump" {
		dumpBlockChain(splits[1])
		return nil, errors.New("Invalid Input")
	}

	cardNumber, err := strconv.Atoi(splits[0])
	if err != nil {
		return nil, errors.New("Invalid Input")
	}

	balanceUpdate, err := strconv.Atoi(splits[1])
	if err != nil {
		return nil, errors.New("Invalid Input")
	}
	if posType {
		if balanceUpdate < 0 {
			fmt.Fprintf(msgB, "%s %s\n", withColor("blue", "<System>:"), "This transaction is not supported at Cash POS Terminal!")
			return nil, errors.New("Invalid Transaction")
		}
	} else {
		if balanceUpdate >= 0 {
			fmt.Fprintf(msgB, "%s %s\n", withColor("blue", "<System>:"), "This transaction is not supported at Retail POS Terminal!")
			return nil, errors.New("Invalid Transaction")
		}
	}

	return &blockchain.Transaction{
		cardNumber, balanceUpdate, "Signer",
	}, nil
}

func dumpBlockChain(fileName string) {
	file, err := os.Create(fmt.Sprintf("%s.txt", fileName))
	if err != nil {
		log.Printf("Error in Opening File")
	}

	for _, block := range blockchain.Blockchain {
		fmt.Fprintf(file, block.DumpToString()+"\n")
	}

}

func showBalance(cardID int, msgB *tview.TextView) {
	fmt.Fprintf(msgB, "%s Balance on Card %v - %v\n", withColor("blue", "<System> :"), cardID, blockchain.Balance[cardID])
}
