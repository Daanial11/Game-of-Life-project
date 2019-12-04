package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

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

var endedTurnWorld [][]uint8

var workerWg sync.WaitGroup

var workerWorld sync.WaitGroup

var evalFin SafeBool

var addedTime SafeBool

var worldEdit SafeBool


func powerCheck(x int) [2]int {
	var remNum [2]int
	y := 16
	for {
		if x > y/2 {
			remNum[0] = y - x
			remNum[1] = y
			return remNum
		}
		y = y / 2
	}
}

func collectNeighbours(x, y int, world [][]byte, height, width int, m bool) int {
	neigh := 0
	for i := -1; i < 2; i++ {
		for j := 0; j < 3; j++ {

			if i != 0 || j != 1 {
				newY := y + j
				newX := x + i
				if newX < 0 {
					newX = width - 1
				}
				if newX == width {
					newX = 0
				}

				if world[newY][newX] == 255 {
					neigh++

				}

			}

		}
	}

	return neigh
}

func alivePrint2(p golParams){
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if endedTurnWorld[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}
	fmt.Println(finalAlive)
}

func timer (p golParams) {
	ticker:=time.NewTicker(2*time.Second)
	for {
		if evalFin.Get(){
			break
		}
		select {
			case <-ticker.C:
				alivePrint(p)
			default:
			}
	}

}

func alivePrint(p golParams) {
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if endedTurnWorld[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}
	fmt.Println("Alive Cells: ", len(finalAlive))
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

//func worker(startY, endY, startX, endX int, data func(y, x int) uint8, p golParams, out chan<- [][]uint8){
func worker(startY, endY, endX, t int, p golParams,  commandChan chan uint8) {
	height := endY - startY
	addedTime.Set(false)

	currentSegment := makeMatrix(height, p.imageWidth)
	segmentCopy:=makeMatrix(height+2,p.imageWidth)
	m:=0
	//copying segment as using the append operations below modifies 'currentSegment'
	for turns := 0; turns < p.turns; turns++ {
		m=0
		column := 0
		if !worldEdit.Get() {
			worldEdit.Set(true)
			for y := 0; y < height+2; y++ {
				if startY+y == 0 {
					column = p.imageHeight - 1
				} else if startY+y > p.imageHeight {
					column = 0
				} else {
					column = startY + y - 1
				}
				segmentCopy[y] = endedTurnWorld[column]
			}
			worldEdit.Set(false)
		}

			//fmt.Println(currentSegment)
		for {
			if !worldEdit.Get() {
				worldEdit.Set(true)
				for y := 0; y < height; y++ {
					for x := 0; x < endX; x++ {
						if t == 1 && p.threads == 2 && x == 4 && y == 0 {
							fmt.Println(collectNeighbours(4, 0, segmentCopy, p.imageHeight, p.imageWidth, false))
						}
						currentSegment[y][x] = GoLogic(segmentCopy[y+1][x], collectNeighbours(x, y, segmentCopy, p.imageHeight, p.imageWidth, false))
					}
				}

				worldEdit.Set(false)
				m++

			}
			if m == p.threads{break}
		}


		/*command := <-commandChan
		if command == '1' {
			if t == 0 {
				fmt.Println("Current turn:", turns)
			}
			select {
			case <-commandChan:
			}

		}

		close(commandChan)

		*/

		for {
			if !worldEdit.Get() {
				worldEdit.Set(true)
				for y := 0; y < height; y++ {
					endedTurnWorld[startY+y] = currentSegment[y]
				}
				worldEdit.Set(false)

				break
			}
		}
	}
	if t==0 {
		//fmt.Println(endedTurnWorld)
	}
	evalFin.Set(true)
	workerWg.Done()

}

func GoLogic(cell byte, aliveNeigh int) byte {
	if aliveNeigh == 3 && cell == 0 {
		return 255
	}
	if aliveNeigh < 2 || aliveNeigh > 3 {
		return 0
	}
	return cell
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, keyChan <-chan rune) {
	evalFin.Set(false)

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	endedTurnWorld = make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				//fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// Calculate the new state of Game of Life after the given number of turns.

	currentHeight := 0

	//Using this to control each worker
	commandChan := make([]chan uint8, p.threads)
	for i := range commandChan {
		commandChan[i] = make(chan uint8)
	}

	powerChecker := powerCheck(p.threads)
	addRowThreads := powerChecker[0]
	dividedHeight := p.imageHeight / powerChecker[1]
	x := p.threads
	for i := 0; i <p.imageHeight; i++ {
			endedTurnWorld[i] = world[i]
	}

	if p.turns > 0 {

		for threads := 0; threads < p.threads; threads++ {
			workerWg.Add(1)

			if addRowThreads == x {
				dividedHeight = dividedHeight * 2
			}
			x--

			go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, threads, p, commandChan[threads])

			currentHeight += dividedHeight
		}

		go timer(p)
		for {
			if evalFin.Get(){
				break
			}
			/*select {
			case key := <-keyChan:
				switch key {
				case 'q':
					command := byte('0')
					for threads := 0; threads < p.threads; threads++ {
						commandChan[threads] <- command
					}
					for i := 0; i < p.imageHeight; i++ {
						for x := 0; x < p.imageWidth; x++ {
							world[i][x] = endedTurnWorld[i][x]
						}
					}
					world = endedTurnWorld
					d.io.command <- ioOutput
					d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
					d.io.outputVal <- world
					d.io.command <- ioCheckIdle
					<-d.io.idle
					fmt.Println("Terminated")
					os.Exit(1)
				case 's':
					command := byte('0')
					for threads := 0; threads < p.threads; threads++ {
						commandChan[threads] <- command
					}
					for i := 0; i < p.imageHeight; i++ {
						for x := 0; x < p.imageWidth; x++ {
							world[i][x] = endedTurnWorld[i][x]
						}
					}

					d.io.command <- ioOutput
					d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
					d.io.outputVal <- world
					d.io.command <- ioCheckIdle
					<-d.io.idle
				case 'p':
					command := byte('1')
					for threads := 0; threads < p.threads; threads++ {
						commandChan[threads] <- command
					}
					select {
					case <-keyChan:
						for threads := 0; threads < p.threads; threads++ {
							commandChan[threads] <- command
						}
						fmt.Println("Continuing")
					}

				}
			default:
				for threads := 0; threads < p.threads; threads++ {
					SafeSend(commandChan[threads], byte('0'))

				}
			}

			 */
		}
	}
	workerWg.Wait()

	//fmt.Println(endedTurnWorld, p.threads)
	//merging segments for final state
	if !worldEdit.Get() {
		worldEdit.Set(true)
		for i := 0; i < p.imageHeight; i++ {
			world[i] = endedTurnWorld[i]
		}

		worldEdit.Set(false)
	}

	//fmt.Println(world)
	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	//tells IO to start outputting
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
	d.io.outputVal <- world
	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}

func SafeSend(ch chan uint8, value uint8) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()

	ch <- value  // panic if ch is closed
	return false // <=> closed = false; return
}
