package main

import "github.com/golang/protobuf/proto"

// Define the P2P response, which is a iterator.

type P2PResponse struct {
    channel chan proto.Message
    acquiredClose bool
}

func NewP2PResponse() (r *P2PResponse) {
    return &P2PResponse{
        channel: make(chan proto.Message, 1),
        acquiredClose: false,
    }
}

// Functions called by sender

func (r *P2PResponse) Push(msg proto.Message) {
    r.channel <- msg
}

func (r *P2PResponse) AcquiredClose() bool {
    return r.acquiredClose
}

func (r *P2PResponse) Close() {
    close(r.channel)
}

// Functions called by receiver

func (r *P2PResponse) AcquireClose() {
    r.acquiredClose = true
}

func (r *P2PResponse) Get() proto.Message {
    if r.acquiredClose {
        return nil
    }

    msg, more := <-r.channel
    if !more {
        return nil
    }
    return msg
}

