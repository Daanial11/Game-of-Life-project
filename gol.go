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
func worker(startY, endY, endX int, p golParams, out chan [][]uint8) {
	height := endY - startY

	currentSegment := <-out
	//copying segment as using the append operations below modifies 'currentSegment'
	segmentCopy := make([][]uint8, len(currentSegment))
	copy(segmentCopy, currentSegment)

	//removing extra top and bottom row
	tempWorld := append(segmentCopy[:0], segmentCopy[1:]...)
	tempWorld = append(tempWorld[:height], tempWorld[height+1:]...)

	//making copy of tempworld with type [][]byte instead of using the tempWorld above, doesn't work without this for some reason.
	tempWorldCopy := make([][]byte, height)
	for i := range tempWorldCopy {
		tempWorldCopy[i] = make([]byte, p.imageWidth)
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

	out <- tempWorldCopy
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
				//fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// Calculate the new state of Game of Life after the given number of turns.

	//Starting goroutine for the number of alive cells every 2 seconds
	go AlivePrint(world, p)

	for turns := 0; turns < p.turns; turns++ {

		if pausedState.Get() {
			fmt.Println("Current turn:", turns)
			for {
				if !pausedState.Get() {
					break
				}
			}

		}

		currentHeight := 0
		dividedHeight := p.imageHeight / p.threads

		out := make([]chan [][]uint8, p.threads)
		for i := range out {
			out[i] = make(chan [][]uint8)
		}

		for threads := 0; threads < p.threads; threads++ {

			segmentWorld := makeMatrix(0, 0)
			lastRow := world[p.imageHeight-1]
			if threads != 0 {
				segmentWorld = append(segmentWorld, world[((threads)*dividedHeight)-1])
			} else {
				segmentWorld = append(segmentWorld, lastRow)

			}
			for i := 0; i < dividedHeight; i++ {
				segmentWorld = append(segmentWorld, world[(threads*dividedHeight)+i])
				if i == dividedHeight-1 {
					if threads != (p.threads - 1) {
						segmentWorld = append(segmentWorld, world[((threads)*dividedHeight)+i+1])
					} else {
						segmentWorld = append(segmentWorld, world[0])
					}

				}
			}

			go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, p, out[threads])

			out[threads] <- segmentWorld

			currentHeight += dividedHeight
		}

		//combining each segment
		newWorld := makeMatrix(0, 0)
		for i := 0; i < p.threads; i++ {
			newSegment := <-out[i]
			newWorld = append(newWorld, newSegment...)

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
		//Check if key 's' has been pressed, generate PGM with current state and end if pressed.

		if endWithCurrentState.Get() {
			d.io.command <- ioOutput
			d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
			d.io.outputVal <- world
			break
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
