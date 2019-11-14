package main

import (
	"fmt"
	"strconv"
	"strings"
)

func collectNeighbours(x, y int, world [][]byte, p golParams) int {
	neigh := 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			//
			if i != 0 || j != 0 {
				newY := y + j
				newX := x + i
				if newX < 0 {
					newX = p.imageWidth - 1
				}
				if newX == p.imageWidth {
					newX = 0
				}
				if newY < 0 {
					newY = p.imageHeight - 1
				}
				if newY == p.imageHeight {
					newY = 0
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

func worker(startY, endY, startX, endX int, data [][]uint8, p golParams, out chan<- [][]uint8) {
	//height:= endY - startY
	//width:= endX - startX
	tempWorld := makeMatrix(p.imageHeight, p.imageWidth)
	//copying world to temp world
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			tempWorld[y][x] = data[y][x]
		}
	}
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			tempWorld[y][x] = GoLogic(tempWorld[y][x], collectNeighbours(x, y, data, p))
		}
	}
	out <- tempWorld
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
	tempWorld := makeMatrix(p.imageHeight, p.imageWidth)
	//copying world to temp world
	copy(tempWorld,world)
	for turns := 0; turns < p.turns; turns++ {
		//workerHeight := p.imageHeight/p.threads
		out := make([]chan [][]uint8, p.threads)
		newWorld := makeMatrix(0, 0)
		for i := range out {
			out[i] = make(chan [][]uint8)
		}
		go worker(0, p.imageHeight, 0, p.imageWidth, world, p, out[0])


		for threads := 0; threads < 1; threads++ {
			newSegment := <-out[threads]
			newWorld = append(newWorld, newSegment...)
		}

		//copying newworld to tempworld
		copy(tempWorld, newWorld)

		copy(world, newWorld)
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
