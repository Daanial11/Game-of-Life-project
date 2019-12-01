package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

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

func collectNeighbours(x, y int, world [][]byte, height, width int) int {
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
					if x == 4 && y == 6 {
						//fmt.Println(newX, newY)
					}
					neigh++

				}

			}

		}
	}

	return neigh
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

//func worker(startY, endY, startX, endX int, data func(y, x int) uint8, p golParams, out chan<- [][]uint8){
func worker(startY, endY, endX, t int, p golParams, out chan uint8, upperCom, lowerCom chan uint8, aliveChan chan int, commandChan chan uint8) {
	height := endY - startY

	tempWorld := makeMatrix(height, p.imageWidth)
	tempWorldCopy := makeMatrix(height, p.imageWidth)

	currentSegment := makeMatrix(height+2, p.imageWidth)

	for y := 0; y < height+2; y++ {
		for x := 0; x < p.imageWidth; x++ {
			currentSegment[y][x] = <-out
		}
	}
	//fmt.Println(currentSegment)
	//fmt.Println(t)

	//copying segment as using the append operations below modifies 'currentSegment'
	segmentCopy := make([][]uint8, len(currentSegment))

	for turns := 0; turns < p.turns; turns++ {

		tempWorld = nil

		copy(segmentCopy, currentSegment)

		//removing extra top and bottom row
		tempWorld = append(segmentCopy[:0], segmentCopy[1:]...)
		tempWorld = append(tempWorld[:height], tempWorld[height+1:]...)

		for y := 0; y < height; y++ {
			for x := 0; x < p.imageWidth; x++ {
				tempWorldCopy[y][x] = tempWorld[y][x]
			}
		}

		aliveCount := 0
		for y := 0; y < height; y++ {
			for x := 0; x < endX; x++ {
				tempWorldCopy[y][x] = GoLogic(tempWorldCopy[y][x], collectNeighbours(x, y, currentSegment, height, p.imageWidth))
				if tempWorldCopy[y][x] == 255 {
					aliveCount++
				}

			}
		}
		select {
		case command := <-commandChan:
			//fmt.Println(command)
			switch command {
			case '1':
				aliveChan <- aliveCount

			case '2':
				for y := 0; y < height; y++ {
					for x := 0; x < p.imageWidth; x++ {
						out <- tempWorldCopy[y][x]
					}
				}
			case '3':
				if t == 0 {
					fmt.Println("Current turn:", turns)
				}
				select {
				case <-commandChan:
				}

			}

		default:
		}

		if t == 0 {

			for x := 0; x < p.imageWidth; x++ {
				lowerCom <- tempWorldCopy[height-1][x]
			}

			for x := 0; x < p.imageWidth; x++ {
				currentSegment[0][x] = <-upperCom
			}

			for i := 1; i <= height; i++ {
				for x := 0; x < p.imageWidth; x++ {
					currentSegment[i][x] = tempWorldCopy[i-1][x]
				}
			}
			for x := 0; x < p.imageWidth; x++ {
				upperCom <- tempWorldCopy[0][x]
			}

			for x := 0; x < p.imageWidth; x++ {
				currentSegment[height+1][x] = <-lowerCom
			}

		} else {

			for x := 0; x < p.imageWidth; x++ {
				currentSegment[0][x] = <-upperCom

			}
			for x := 0; x < p.imageWidth; x++ {
				lowerCom <- tempWorldCopy[height-1][x]
			}

			for i := 1; i <= height; i++ {
				for x := 0; x < p.imageWidth; x++ {
					currentSegment[i][x] = tempWorldCopy[i-1][x]
				}
			}

			for x := 0; x < p.imageWidth; x++ {
				currentSegment[height+1][x] = <-lowerCom
			}
			for x := 0; x < p.imageWidth; x++ {
				upperCom <- tempWorldCopy[0][x]
			}

		}

	}

	if t == 0 {
		commandChan <- byte('4')
	}

	for y := 0; y < height; y++ {
		for x := 0; x < p.imageWidth; x++ {
			out <- tempWorldCopy[y][x]
		}
	}
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

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
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

	out := make([]chan uint8, p.threads)
	for i := range out {
		out[i] = make(chan uint8)
	}
	//Using this to control each worker
	commandChan := make([]chan uint8, p.threads)
	for i := range out {
		commandChan[i] = make(chan uint8, 1)
	}
	//Used for halo communication between workers
	workerCom := make([]chan uint8, p.threads)
	for i := range workerCom {
		workerCom[i] = make(chan uint8)
	}
	//Used to send alive count from worker to distributor
	aliveChan := make(chan int, 8)

	powerChecker := powerCheck(p.threads)
	addRowThreads := powerChecker[0]
	dividedHeight := p.imageHeight / powerChecker[1]
	x := p.threads

	if p.turns > 0 {
		for threads := 0; threads < p.threads; threads++ {

			if addRowThreads == x {
				dividedHeight = dividedHeight * 2
			}
			x--
			if threads == 0 {
				go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, threads, p, out[threads], workerCom[p.threads-1], workerCom[0], aliveChan, commandChan[threads])
			} else {
				go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, threads, p, out[threads], workerCom[threads-1], workerCom[threads], aliveChan, commandChan[threads])
			}

			lastRow := world[p.imageHeight-1]
			if threads != 0 {

				for x := 0; x < p.imageWidth; x++ {
					out[threads] <- world[currentHeight-1][x]
				}
			} else {

				for x := 0; x < p.imageWidth; x++ {
					out[threads] <- lastRow[x]
				}

			}
			for i := 0; i < dividedHeight; i++ {

				for x := 0; x < p.imageWidth; x++ {
					out[threads] <- world[currentHeight+i][x]
				}
				if i == dividedHeight-1 {
					if threads != (p.threads - 1) {

						for x := 0; x < p.imageWidth; x++ {
							out[threads] <- world[currentHeight+i+1][x]
						}
					} else {

						for x := 0; x < p.imageWidth; x++ {
							out[threads] <- world[0][x]
						}
					}

				}
			}

			currentHeight += dividedHeight
		}

		ticker := time.NewTicker(2 * time.Second)
		workersFinished := false
		for {
			if workersFinished {
				break
			}
			select {
			case <-ticker.C:
				command := byte('1')
				aliveCount := 0
				for threads := 0; threads < p.threads; threads++ {
					commandChan[threads] <- command
				}

				for threads := 0; threads < p.threads; threads++ {
					aliveCount += <-aliveChan
				}

				fmt.Println("Alive cells:", aliveCount)

			default:
			}

			select {
			case key := <-keyChan:
				switch key {
				case 'q':

					command := byte('2')
					for threads := 0; threads < p.threads; threads++ {
						commandChan[threads] <- command
					}
					dividedHeight2 := p.imageHeight / powerChecker[1]
					x2 := p.threads
					currentHeight2 := 0
					for threads := 0; threads < p.threads; threads++ {
						if addRowThreads == x2 {
							dividedHeight2 = dividedHeight2 * 2
						}
						x2--
						for i := 0; i < dividedHeight2; i++ {
							for x := 0; x < p.imageWidth; x++ {
								cell := <-out[threads]
								world[currentHeight2+i][x] = byte(cell)
							}
						}
						currentHeight2 += dividedHeight2
					}
					d.io.command <- ioOutput
					d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
					d.io.outputVal <- world
					d.io.command <- ioCheckIdle
					<-d.io.idle
					fmt.Println("Terminated")
					os.Exit(1)
				case 's':
					command := byte('2')
					for threads := 0; threads < p.threads; threads++ {
						commandChan[threads] <- command
					}
					dividedHeight2 := p.imageHeight / powerChecker[1]
					x2 := p.threads
					currentHeight2 := 0
					for threads := 0; threads < p.threads; threads++ {
						if addRowThreads == x2 {
							dividedHeight2 = dividedHeight2 * 2
						}
						x2--
						for i := 0; i < dividedHeight2; i++ {
							for x := 0; x < p.imageWidth; x++ {
								cell := <-out[threads]
								world[currentHeight2+i][x] = byte(cell)
							}
						}
						currentHeight2 += dividedHeight2
					}
					d.io.command <- ioOutput
					d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
					d.io.outputVal <- world
					d.io.command <- ioCheckIdle
					<-d.io.idle
				case 'p':
					command := byte('3')
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
			}

			select {
			case <-commandChan[0]:
				workersFinished = true
			default:
			}

		}

		//merging segments for final state
		dividedHeight2 := p.imageHeight / powerChecker[1]
		x2 := p.threads
		currentHeight2 := 0

		for threads := 0; threads < p.threads; threads++ {
			if addRowThreads == x2 {
				dividedHeight2 = dividedHeight2 * 2
			}
			x2--
			for i := 0; i < dividedHeight2; i++ {
				for x := 0; x < p.imageWidth; x++ {
					cell := <-out[threads]
					world[currentHeight2+i][x] = byte(cell)
				}
			}
			currentHeight2 += dividedHeight2
		}

	}
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
	//fmt.Println(finalAlive)

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
