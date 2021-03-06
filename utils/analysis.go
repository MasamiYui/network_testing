package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"path"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/types"
)

var billion = 1000000000

type timeSlice []time.Time

func (p timeSlice) Len() int {
	return len(p)
}

func (p timeSlice) Less(i, j int) bool {
	return p[i].Before(p[j])
}

func (p timeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func main() {
	args := os.Args[1:]
	if len(args) < 5 {
		fmt.Println("transact.go expects five args (datadir, nVals, nTxsCommited, nTxsExpected, start block, end block)")
		os.Exit(1)
	}

	dataDir, nValsString, nTxsCommittedString, nTxsExpectedString, startHeightString, endHeightString := args[0], args[1], args[2], args[3], args[4], args[5]
	nVals, err := strconv.Atoi(nValsString)
	if err != nil {
		fmt.Println("nVals must be an integer:", err)
		os.Exit(1)
	}
	nTxsCommitted, err := strconv.Atoi(nTxsCommittedString)
	if err != nil {
		fmt.Println("nTxs must be an integer:", err)
		os.Exit(1)
	}
	nTxsExpected, err := strconv.Atoi(nTxsExpectedString)
	if err != nil {
		fmt.Println("nTxs must be an integer:", err)
		os.Exit(1)
	}
	startHeight, err := strconv.Atoi(startHeightString)
	if err != nil {
		fmt.Println("startHeight must be an integer:", err)
		os.Exit(1)
	}
	endHeight, err := strconv.Atoi(endHeightString)
	if err != nil {
		fmt.Println("endHeight must be an integer:", err)
		os.Exit(1)
	}

	fmt.Printf("Grabbing block times for %d validators ... \n", nVals)
	// list of times for each validator, for each block
	nBlocks := endHeight - startHeight + 1
	valBlockTimes := make([][]time.Time, nBlocks)
	for i := 1; i <= nVals; i++ {
		fmt.Printf("%d ", i)
		b, err := ioutil.ReadFile(path.Join(dataDir, fmt.Sprintf("%d", i), "cswal"))
		if err != nil {
			fmt.Println("error reading cswal", err)
			os.Exit(1)
		}

		blockN := 0
		lines := strings.Split(string(b), "\n")
		fmt.Println("Blocktimes:")
	INNER:
		for lineNum, l := range lines {
			if len(l) <= 1 {
				fmt.Printf("WARN: cswal (val%d) with empty line (%d)\n", i, lineNum)
				continue
			}
			var err error
			var msg consensus.ConsensusLogMessage
			wire.ReadJSON(&msg, []byte(l), &err)
			if err != nil {
				fmt.Printf("Error reading json data from cswal for val %d (1-based index) on line %d: %v\n", i, lineNum, err)
				fmt.Println(l)
				os.Exit(1)
			}

			m, ok := msg.Msg.(types.EventDataRoundState)
			if !ok {
				continue INNER
			} else if m.Step != consensus.RoundStepCommit.String() {
				continue INNER
			} else if m.Height < startHeight {
				continue INNER
			} else if m.Height > endHeight {
				break INNER
			}
			fmt.Println(msg.Time)
			valBlockTimes[blockN] = append(valBlockTimes[blockN], msg.Time)
			blockN += 1
		}
		fmt.Println("")
	}
	fmt.Printf("\n")

	twoThirdth := nVals * 2 / 3 // plus one but this is used as an index into a slice

	fmt.Printf("Sorting %d block times ... \n", nBlocks)
	var latencyCum time.Duration
	var lastBlockTime time.Time
	// now loop through blocks, sort times across validators, grab 2/3th val as official time
	for i := 0; i < nBlocks; i++ {
		fmt.Printf("%d ", i)
		sort.Sort(timeSlice(valBlockTimes[i]))
		blockTime := valBlockTimes[i][twoThirdth]
		if i == 0 {
			lastBlockTime = blockTime
			continue
		}

		diff := blockTime.Sub(lastBlockTime)
		latencyCum += diff
		lastBlockTime = blockTime
	}
	fmt.Printf("\n\n")

	latency := float64(latencyCum) / float64(endHeight-startHeight) / float64(billion)
	throughput := float64(nTxsCommitted) / (float64(latencyCum) / float64(billion))
	fractionCommitted := float64(nTxsCommitted) / float64(nTxsExpected)
	fmt.Println("Mean latency", latency)
	fmt.Println("Throughput", throughput)
	fmt.Println("Fraction committed:", fractionCommitted)

	results := fmt.Sprintf("%f,%f,%f", latency, throughput, fractionCommitted)
	if err := ioutil.WriteFile(path.Join(dataDir, "final_results"), []byte(results), 0666); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
