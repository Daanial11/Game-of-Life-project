package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

//As we are accessing world in two places (distributor for giving world the final state and here) we need a
//mutex lock to prevent a data race which would lead to undefined behaviour
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

func AlivePrint(world [][]uint8, p golParams) {
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ticker.C:
			var Alive []cell
			// Go through the world and append the cells that are still alive.
			if !worldEdit.Get() {
				worldEdit.Set(true)
				for y := 0; y < p.imageHeight; y++ {
					for x := 0; x < p.imageWidth; x++ {
						if world[y][x] != 0 {
							Alive = append(Alive, cell{x: x, y: y})
						}
					}
				}
				worldEdit.Set(false)
				fmt.Println("Alive cells: ", len(Alive))
			}
			//Checks if processing is paused or not
		default:
			if pausedState.Get() {
				for {
					if !pausedState.Get() {
						break
					}
				}

			}
		}

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
func worker(startY, endY, endX, t int, p golParams, out chan uint8, upperCom, lowerCom chan uint8) {
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

		if pausedState.Get() {
			fmt.Println("Current turn:", turns)
			for {
				if !pausedState.Get() {
					break
				}
			}

		}
		for y := 0; y < height; y++ {
			for x := 0; x < p.imageWidth; x++ {
				tempWorldCopy[y][x] = tempWorld[y][x]
			}
		}

		for y := 0; y < height; y++ {
			for x := 0; x < endX; x++ {
				tempWorldCopy[y][x] = GoLogic(tempWorldCopy[y][x], collectNeighbours(x, y, currentSegment, height, p.imageWidth))

			}
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
	fmt.Println("test")
	for y := 0; y < height; y++ {
		for x := 0; x < p.imageWidth; x++ {
			out <- tempWorldCopy[y][x]
		}
		//out <- tempWorldCopy
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
func distributor(p golParams, d distributorChans, alive chan []cell) {

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
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// Calculate the new state of Game of Life after the given number of turns.

	//Starting goroutine for the number of alive cells every 2 seconds
	go AlivePrint(world, p)

	currentHeight := 0

	out := make([]chan uint8, p.threads)
	for i := range out {
		out[i] = make(chan uint8)
	}

	workerCom := make([]chan uint8, p.threads)
	for i := range workerCom {
		workerCom[i] = make(chan uint8)
	}

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
				go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, threads, p, out[threads], workerCom[p.threads-1], workerCom[0])
			} else {
				go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, threads, p, out[threads], workerCom[threads-1], workerCom[threads])
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

		//combining each segment

		newWorld := makeMatrix(p.imageHeight, p.imageWidth)
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				i := y / (p.imageHeight / p.threads)
				newWorld[y][x] = <-out[i]
			}
		}

		//Copying over the final world state for this turn, using mutex to avoid data race with aliveprint function

		if !worldEdit.Get() {
			worldEdit.Set(true)
			for y := 0; y < p.imageHeight; y++ {
				for x := 0; x < p.imageWidth; x++ {
					world[y][x] = newWorld[y][x]
				}
			}
			worldEdit.Set(false)
		}
	}

	//Check if key 's' has been pressed, generate PGM with current state.

	//Check if key 's' has been pressed, generate PGM with current state and end if pressed.

	if genCurrentState.Get() {
		d.io.command <- ioOutput
		d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
		d.io.outputVal <- world

		for {
			if !genCurrentState.Get() {
				break
			}
		}

	}

	if terminate.Get() {
		d.io.command <- ioOutput
		d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
		if !worldEdit.Get() {
			worldEdit.Set(true)
			d.io.outputVal <- world
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
