package blockchain

import (
	"context"
	"encoding/json"
	"log"
	"time"

	peer "github.com/libp2p/go-libp2p-peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const ChatRoomBufSize = 128

type ChatRoom struct {
	Messages chan UIMessage
	Ctx      context.Context
	ps       *pubsub.PubSub
	topic    *pubsub.Topic
	sub      *pubsub.Subscription

	roomName string
	self     peer.ID
}

type ChatMessage struct {
	Type             int
	SenderID         string
	IntendedReceiver string
	Message          json.RawMessage
}

type LatestIndexMessage struct {
	LatestIndex int `json:"latestIndex"`
}

type UIMessage struct {
	SenderID string
	Message  string
}

func JoinChatRoom(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, roomName string) (*ChatRoom, error) {
	topic, err := ps.Join(topicName(roomName))
	if err != nil {
		return nil, err
	}
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	cr := &ChatRoom{
		Ctx:      ctx,
		ps:       ps,
		topic:    topic,
		sub:      sub,
		self:     selfID,
		roomName: roomName,
		Messages: make(chan UIMessage, ChatRoomBufSize),
	}

	time.Sleep(2 * time.Second)
	cr.RequestIndices()
	maxIndexPeer := cr.readIndices()
	log.Printf("MaxIndex : %v ", maxIndexPeer)
	if maxIndexPeer != "" {
		cr.RequestBlock(maxIndexPeer) // from Largest chain holder
	}

	go cr.readLoop()
	return cr, nil
}

func (cr *ChatRoom) RequestIndices() {
	m := ChatMessage{
		Type:     1,
		SenderID: cr.self.Pretty(),
	}
	jsonM, err := json.Marshal(m)
	if err != nil {
		log.Printf("RequestIndices: %s", err)
	}
	err = cr.topic.Publish(cr.Ctx, jsonM)
	if err != nil {
		log.Printf("RequestIndices: %s", err)
	}
}

func (cr *ChatRoom) RequestBlock(receiver string) error {
	log.Printf("Inside Request Block")
	m := ChatMessage{
		Type:             5,
		SenderID:         cr.self.Pretty(),
		IntendedReceiver: receiver,
	}

	jsonM, err := json.Marshal(m)
	if err != nil {
		return err
	}
	log.Printf("Request Block Function Ended")
	return cr.topic.Publish(cr.Ctx, jsonM)
}

func (cr *ChatRoom) readIndices() string {
	log.Println("Reading Indeices")
	peersList := cr.ListPeers()
	peerIndex := make(map[string]int)

	for i := 0; i < len(peersList); { //T
		msg, err := cr.sub.Next(cr.Ctx)
		if err != nil {
			log.Printf("%s", msg)
		}

		log.Printf("Received : %v", string(msg.Data))

		cm := new(ChatMessage)
		err = json.Unmarshal(msg.Data, cm)
		if err != nil {
			log.Printf("Unmarshal Error %v", err)
			continue
		}

		if msg.ReceivedFrom == cr.self {
			continue
		}

		var latestIndex LatestIndexMessage
		err = json.Unmarshal(cm.Message, &latestIndex)
		if err != nil {
			log.Printf("Error in ReadIndex %s", err)
			continue
		}
		i++
		log.Printf("Read IndexReturn Type msg from %s", msg.ReceivedFrom.Pretty())
		peerIndex[cm.SenderID] += latestIndex.LatestIndex
	}

	maxTillNow := -1
	var indexMax string
	for key, val := range peerIndex {
		if val > maxTillNow {
			maxTillNow = val
			indexMax = key
		}
	}
	return indexMax
}

func (cr *ChatRoom) Publish(trans Transaction) error {

	block, err := generateBlock(getLastBlock(), trans)
	if err != nil {
		return err
	}

	jsonBlock, err := json.Marshal(block)
	if err != nil {
		log.Printf("Publish : Error Marshaling Json %s", jsonBlock)
	}

	m := ChatMessage{
		Message:  jsonBlock,
		SenderID: cr.self.Pretty(),
		Type:     3,
	}

	msgBytes, err := json.Marshal(m)

	if err != nil {
		log.Printf("Publish : Error Marshaling Json %s", jsonBlock)
	}
	return cr.topic.Publish(cr.Ctx, msgBytes)
}

func (cr *ChatRoom) ListPeers() []peer.ID {
	return cr.ps.ListPeers(topicName(cr.roomName))
}

//BreakDown Into Function
func (cr *ChatRoom) readLoop() {
	log.Println("Inside readLoop")
	for {
		msg, err := cr.sub.Next(cr.Ctx)
		if err != nil {
			close(cr.Messages)
			return
		}
		log.Printf("Received : %v", string(msg.Data))

		cm := new(ChatMessage)
		err = json.Unmarshal(msg.Data, cm)
		if err != nil {
			log.Printf("Unmarshal Error %v", err)
			continue
		}

		//Message not intended to me continue
		if cm.IntendedReceiver != "" && cm.IntendedReceiver != cr.self.Pretty() {
			continue
		}

		switch cm.Type {
		case 1:
			li := LatestIndexMessage{
				LatestIndex: len(Blockchain),
			}
			jsonLi, err := json.Marshal(li)
			if err != nil {
				log.Println(err)
			}

			// Create Chat Message

			reply := ChatMessage{
				IntendedReceiver: cm.SenderID,
				Message:          jsonLi,
				SenderID:         cr.self.Pretty(),
				Type:             2,
			}

			jsonMsg, err := json.Marshal(reply)
			if err != nil {
				log.Println(err)
			}
			cr.topic.Publish(cr.Ctx, jsonMsg)

		case 3:
			block := new(Block)
			err := json.Unmarshal(cm.Message, block)
			if err != nil {
				log.Printf("%s", err)
			}
			log.Printf("BlockRec %s", block.DumpToString())
			validateAndAdd(*block)

			if msg.ReceivedFrom == cr.self {
				continue
			}
			cr.Messages <- UIMessage{cm.SenderID, block.Trans.AsString()}

		case 4:
			//Received full BlockChain
			// log.Panicln("Recieved Full BlockChain")
			newBlockChain := new([]Block)
			err := json.Unmarshal(cm.Message, newBlockChain)
			if err != nil {
				log.Println(err)
			}
			Blockchain = *newBlockChain
			generateNewBalanceMap()

		case 5: //Request BlockChain Message Received
			if cm.IntendedReceiver != cr.self.Pretty() {
				continue
			}

			mutex.Lock()
			payload, err := json.Marshal(Blockchain)
			mutex.Unlock()
			if err != nil {
				log.Printf("Error in Marshalling BlockChain %s ", err)
			}

			reply := ChatMessage{
				IntendedReceiver: cm.SenderID,
				Message:          payload,
				Type:             4,
				SenderID:         cr.self.Pretty(),
			}

			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Printf("Error in Marshalling BlockChainMsg %s ", err)
			}
			cr.topic.Publish(cr.Ctx, jsonReply)
		}

	}
}

func topicName(roomName string) string {
	return "chat-room" + roomName
}
