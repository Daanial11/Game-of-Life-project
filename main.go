package main

import (
	"flag"
	"fmt"
	"sync"
)

// golParams provides the details of how to run the Game of Life and which image to load.
type golParams struct {
	turns       int
	threads     int
	imageWidth  int
	imageHeight int
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
	ioCheckIdle
)

// cell is used as the return type for the testing framework.
type cell struct {
	x, y int
}

// distributorToIo defines all chans that the distributor goroutine will have to communicate with the io goroutine.
// Note the restrictions on chans being send-only or receive-only to prevent bugs.
type distributorToIo struct {
	command chan<- ioCommand
	idle    <-chan bool

	filename  chan<- string
	inputVal  <-chan uint8
	outputVal chan<- [][]byte
}

// ioToDistributor defines all chans that the io goroutine will have to communicate with the distributor goroutine.
// Note the restrictions on chans being send-only or receive-only to prevent bugs.
type ioToDistributor struct {
	command <-chan ioCommand
	idle    chan<- bool

	filename  <-chan string
	inputVal  chan<- uint8
	outputVal <-chan [][]byte
}

// distributorChans stores all the chans that the distributor goroutine will use.
type distributorChans struct {
	io distributorToIo
}

// ioChans stores all the chans that the io goroutine will use.
type ioChans struct {
	distributor ioToDistributor
}

// gameOfLife is the function called by the testing framework.
// It makes some channels and starts relevant goroutines.
// It places the created channels in the relevant structs.
// It returns an array of alive cells returned by the distributor.
func gameOfLife(p golParams, keyChan <-chan rune) []cell {
	var dChans distributorChans
	var ioChans ioChans

	ioCommand := make(chan ioCommand)
	dChans.io.command = ioCommand
	ioChans.distributor.command = ioCommand

	ioIdle := make(chan bool)
	dChans.io.idle = ioIdle
	ioChans.distributor.idle = ioIdle

	ioFilename := make(chan string)
	dChans.io.filename = ioFilename
	ioChans.distributor.filename = ioFilename

	inputVal := make(chan uint8)
	dChans.io.inputVal = inputVal
	ioChans.distributor.inputVal = inputVal

	outputVal := make(chan [][]byte)
	dChans.io.outputVal = outputVal
	ioChans.distributor.outputVal = outputVal

	aliveCells := make(chan []cell)

	go distributor(p, dChans, aliveCells)
	go pgmIo(p, ioChans)
	go keyButtonControl(keyChan, p, ioChans, dChans)
	alive := <-aliveCells
	return alive
}

//Using safebool and mutex locks to prevent data race that occurs normally when trying to access these variables in multiple places
var pausedState SafeBool
var terminate SafeBool
var genCurrentState SafeBool

//Infinite loop to see what key is pressed running on separate goroutine from logic so commands are nearly instant
func keyButtonControl(keyChan <-chan rune, p golParams, i ioChans, d distributorChans) {

	for {
		select {
		case key := <-keyChan:
			switch key {
			case 'q':
				terminate.Set(true)
			case 's':
				genCurrentState.Set(true)
			case 'p':
				if !pausedState.Get() {
					pausedState.Set(true)
				} else {
					pausedState.Set(false)
					fmt.Println("Continuing")
				}

			}
		}
	}
}

type SafeBool struct {
	val bool
	m   sync.Mutex
}

//Locks when getting the value and unlocks after
func (i *SafeBool) Get() bool {
	i.m.Lock()
	defer i.m.Unlock()
	return i.val
}

//Locks when setting the value and unlocks after
func (i *SafeBool) Set(val bool) {
	i.m.Lock()
	defer i.m.Unlock()
	i.val = val
}

// main is the function called when starting Game of Life with 'make gol'
// Do not edit until Stage 2.
func main() {
	var params golParams

	keyChanSend := make(chan rune)

	flag.IntVar(
		&params.threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.imageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.imageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.Parse()

	params.turns = 1000000

	startControlServer(params)
	go getKeyboardCommand(keyChanSend)
	gameOfLife(params, keyChanSend)

	StopControlServer()
}
