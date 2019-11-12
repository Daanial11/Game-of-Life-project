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

/*func aliveNeighCount(neigh []byte) int {
	aliveCount := 0
	for _, cell := range neigh {
		if cell != 0 {
			aliveCount++
		}
	}
	return aliveCount
}*/

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
	for turns := 0; turns < p.turns; turns++ {
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
		//using fixed tempworld state to check and update world
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				temp := collectNeighbours(x, y, tempWorld, p)

				if temp == 3 && world[y][x] == 0 {
					world[y][x] = 255
				}
				if temp < 2 || temp > 3 {
					world[y][x] = 0
				}

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
