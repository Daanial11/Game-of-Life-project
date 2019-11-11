package main

import (
	"fmt"
	"strconv"
	"strings"
)

func collectNeighbours(x,y int, world [][]byte, p golParams) []byte{
	neigh := make([]byte, 8)
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			if i != 1 && j != 1 {
				newY :=y-1+i
				newX :=x-1+j
				if newX < 0 { newX = p.imageWidth - 1}
				if newY < 0 { newY = p.imageHeight - 1}
				if newX == p.imageWidth { newX = 0}
				if newY == p.imageHeight { newY = 0}
				neigh = append(neigh, world[newY][newX])
			}
		}
	}

	return neigh
}
func aliveNeighCount(neigh[]byte) int{
	aliveCount := 0
	for _, cell := range neigh{
		if cell != 0 {aliveCount++}
	}
	return aliveCount
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
	for turns := 0; turns < p.turns; turns++ {
		tempWorld := make([][]byte, p.imageHeight)
		for turns := 0; turns < p.turns; turns++ {
			copy(tempWorld, world)
			for y := 0; y < p.imageHeight; y++ {
				for x := 0; x < p.imageWidth; x++ {
					temp := aliveNeighCount(collectNeighbours(x, y, world, p))
					if temp < 3 {
						tempWorld[y][x]= 0
					}
					if temp > 4 {
						tempWorld[y][x]= 0
					}
					if temp == 3 {
						tempWorld[y][x] = 255
					}
				}
			}
			copy(world, tempWorld)
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

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
