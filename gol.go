package main

import (
	"fmt"
	"strconv"
	"strings"
)

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
				//if newY < 0 {
				//	newY = height - 1
				//}
				//if newY == height {
				//	newY = 0
				//}

				if world[newY][newX] == 255 {
					neigh++

				}

			}

		}
	}

	return neigh
}

func makeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

//func worker(startY, endY, startX, endX int, data func(y, x int) uint8, p golParams, out chan<- [][]uint8){
func worker(startY, endY, endX int, p golParams, out chan [][]uint8, turns int) {
	height := endY - startY

	currentSegment := <-out
	//copying segment as using the append operatins below modifies 'currentSegment'
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
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// Calculate the new state of Game of Life after the given number of turns.
	tempWorld := make([][]byte, p.imageHeight)
	for i := range tempWorld {
		tempWorld[i] = make([]byte, p.imageWidth)
	}
	//copying world to temp world
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			tempWorld[y][x] = world[y][x]
		}
	}

	for turns := 0; turns < p.turns; turns++ {

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

			go worker(currentHeight, currentHeight+dividedHeight, p.imageWidth, p, out[threads], turns)

			out[threads] <- segmentWorld

			currentHeight += dividedHeight
		}

		//combining each segment
		newWorld := makeMatrix(0, 0)
		for i := 0; i < p.threads; i++ {
			newSegment := <-out[i]
			newWorld = append(newWorld, newSegment...)

		}
		//Copying over the final world state for this turn
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = newWorld[y][x]
			}
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
