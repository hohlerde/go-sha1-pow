package main

import (
	"crypto/sha1"
	"hash"
	"log"
	"math/big"
	"math/bits"
	"os"
	"sync"
	"time"

	flag "github.com/spf13/pflag"
)

type finishData struct {
	elapsed  time.Duration
	solution string
}

var (
	prefix        string
	difficulty    int
	maxGoroutines int
	compRange     int
	debug         bool

	doneLock  sync.Mutex
	wgWaiting = false
)

func generateSHA1(prefix []byte, nonce *big.Int, scha1 hash.Hash) ([]byte, error) {
	scha1.Reset()

	_, err := scha1.Write(prefix)
	if err != nil {
		return nil, err
	}

	_, err = scha1.Write([]byte(nonce.Text(16)))
	if err != nil {
		return nil, err
	}

	return scha1.Sum(nil), nil
}

func isValidChecksum(hashBytes []byte, difficulty int) bool {
	k := difficulty/2 + difficulty%2
	zeroes := 0
	for i := 0; i < k; i++ {
		if hashBytes[i] == 0 {
			zeroes += 2
		} else {
			bitz := bits.LeadingZeros8(hashBytes[i])
			zeroes += bitz / 4
			break
		}
	}
	if zeroes >= difficulty {
		return true
	}
	return false
}

func worker(id *big.Int, prefix []byte, difficulty int, start *big.Int, base *big.Int, debug bool, stop <-chan struct{}, success chan<- string, done chan<- struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	if debug {
		log.Printf("Goroutine %d starting", id)
	}
	scha1 := sha1.New()
	nonce := big.NewInt(0).SetBytes(start.Bytes())
	one := big.NewInt(1)
	end := start.Add(start, base)
	sub := big.NewInt(0)

	var err error
	var hashBytes []byte

	found := false
	for {
		select {
		case <-stop:
			if debug {
				log.Printf("Stopping worker: %d", id)
			}
			return
		default:
		}

		if !found {
			hashBytes, err = generateSHA1(prefix, nonce, scha1)
			if err != nil {
				log.Printf("%s\n", err)
				return
			}
		}

		if found || isValidChecksum(hashBytes, difficulty) {
			if !found {
				if debug {
					log.Printf("Worker %d found suffix\n", id)
				}
			}
			found = true
			select {
			case success <- nonce.Text(16):
				return
			default:
			}
		}

		if !found {
			nonce = nonce.Add(nonce, one)
			sub = sub.Sub(nonce, end)

			if sub.Int64() == 0 {
				var s struct{}
				if debug {
					log.Printf("Worker is done: %d", id)
				}
				done <- s
				return
			}
		}
	}
}

func partitioner(counter *big.Int, base *big.Int, debug bool, done chan struct{}, drain <-chan struct{}, prefix []byte, difficulty int, stop <-chan struct{}, stopWorker <-chan struct{}, success chan<- string, wg *sync.WaitGroup) {
	drainFlag := false
	for {
		select {
		case <-drain:
			drainFlag = true
		case <-stop:
			return
		default:
		}
		<-done
		doneLock.Lock()
		if !drainFlag && !wgWaiting {
			counter = counter.Add(counter, big.NewInt(1))
			if debug {
				log.Printf("Starting new worker: %d", counter)
			}
			start := big.NewInt(0)
			start = start.Mul(counter, base)
			wg.Add(1)
			go worker(big.NewInt(0).SetBytes(counter.Bytes()), prefix, difficulty, start, base, debug, stopWorker, success, done, wg)
		}
		doneLock.Unlock()
	}
}

func createPartition(base *big.Int, noOfRoutines int) []*big.Int {
	parts := make([]*big.Int, noOfRoutines)

	for i := 0; i < noOfRoutines; i++ {
		bigInt := big.NewInt(0)
		bigInt = bigInt.Mul(base, big.NewInt(int64(i)))
		parts[i] = bigInt
	}

	return parts
}

func awaitSolution(startTime time.Time, finish chan finishData, success chan string, stop chan<- struct{}, stopWorker chan<- struct{}, drain chan<- struct{}, done chan<- struct{}, wg *sync.WaitGroup) {
	solution := <-success
	elapsed := time.Since(startTime)

	// here, we can already process the result

	log.Printf("Solution: %s\n", solution)

	close(drain)
	close(stopWorker)
	log.Printf("Waiting for all goroutines to stop...")
	doneLock.Lock()
	wgWaiting = true
	doneLock.Unlock()
	wg.Wait()
	log.Printf("...done")
	close(success)
	close(stop)
	close(done)

	finish <- finishData{solution: solution, elapsed: elapsed}
}

func initFlags() {
	flag.StringVar(&prefix, "prefix", "", "the prefix")
	flag.IntVar(&difficulty, "difficulty", 9, "the difficulty")
	flag.IntVar(&maxGoroutines, "max-workers", 100000, "max number of goroutines to use")
	flag.IntVar(&compRange, "calc-range", 10000000, "the number of calculations for one goroutine")
	flag.BoolVar(&debug, "debug", true, "if false, console output is omitted")
}

func main() {
	initFlags()
	flag.Parse()

	if prefix == "" {
		log.Printf("error: prefix is required")
		os.Exit(1)
	}
	if maxGoroutines <= 0 {
		log.Printf("error: max-workers must be positive > 0")
		os.Exit(1)
	}
	if compRange <= 0 {
		log.Printf("error: calc-range must be positive > 0")
		os.Exit(1)
	}
	if difficulty <= 0 {
		log.Printf("error: difficulty must be positive > 0")
		os.Exit(1)
	}

	prefixBytes := []byte(prefix)
	base := big.NewInt(int64(compRange))

	stopChan := make(chan struct{})
	drainChan := make(chan struct{})
	stopWorkerChan := make(chan struct{})
	successChan := make(chan string)
	doneChan := make(chan struct{})
	finishChan := make(chan finishData)

	start := time.Now()
	wg := &sync.WaitGroup{}

	partition := createPartition(base, maxGoroutines)

	wg.Add(len(partition))

	go awaitSolution(start, finishChan, successChan, stopChan, stopWorkerChan, drainChan, doneChan, wg)
	go partitioner(big.NewInt(int64(maxGoroutines)), base, debug, doneChan, drainChan, prefixBytes, difficulty, stopChan, stopWorkerChan, successChan, wg)

	for i, bigInt := range partition {
		go worker(big.NewInt(int64(i)), prefixBytes, difficulty, bigInt, base, debug, stopWorkerChan, successChan, doneChan, wg)
	}

	fd := <-finishChan

	log.Printf("Using %d worker(s)", maxGoroutines)
	log.Printf("Prefix string: %s\n", prefix)
	log.Printf("Suffix string: %s\n", fd.solution)
	scha1 := sha1.New()
	b := big.NewInt(0)
	b, _ = b.SetString(fd.solution, 16)
	h, _ := generateSHA1(prefixBytes, b, scha1)
	log.Printf("SHA1: % x\n", h)
	log.Printf("Time elapsed: %s\n", fd.elapsed)
}
