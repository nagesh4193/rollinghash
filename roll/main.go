package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"gopkg.in/chmduquesne/rollinghash.v2/rabinkarp32"

	"code.cloudfoundry.org/bytefmt"
	"github.com/chmduquesne/rollinghash"
	_adler32 "github.com/chmduquesne/rollinghash/adler32"
	"github.com/chmduquesne/rollinghash/buzhash32"
	"github.com/chmduquesne/rollinghash/buzhash64"
)

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB

	clearscreen = "\033[2J\033[1;1H"
	clearline   = "\x1b[2K"
)

func genMasks() (res []uint64) {
	res = make([]uint64, 64)
	ones := ^uint64(0) // 0xffffffffffffffff
	for i := 0; i < 64; i++ {
		res[i] = ones >> uint(63-i)
	}
	return
}

func hash2uint64(s []byte) (res uint64) {
	for _, b := range s {
		res <<= 8
		res |= uint64(b)
	}
	return
}

func main() {
	rollsum := flag.String("sum", "adler32", "adler32|rabinkarb32|buzhash32|buzhash64")
	dostats := flag.Bool("stats", false, "Do some stats about the rolling sum")
	size := flag.String("size", "256M", "How much data to read")
	flag.Parse()

	fileSize, err := bytefmt.ToBytes(*size)
	if err != nil {
		log.Fatal(err)
	}

	bufsize := 1 * MiB
	rbuf := make([]byte, bufsize)
	hbuf := make([]byte, 0, 8)
	t := time.Now()

	f, err := os.Open("/dev/urandom")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	io.ReadFull(f, rbuf)

	var roll rollinghash.Hash
	switch *rollsum {
	case "adler32":
		roll = _adler32.New()
	case "bozo32":
		roll = rabinkarp32.New()
	case "buzhash32":
		roll = buzhash32.New()
	case "buzhash64":
		roll = buzhash64.New()
	default:
		log.Fatalf("%s: unrecognized checksum", *rollsum)
	}
	roll.Write(rbuf[:64])

	masks := genMasks()
	hits := make(map[uint64]uint64)
	for _, m := range masks {
		hits[m] = 0
	}

	n := uint64(0)
	k := 0
	for n < fileSize {
		if k >= bufsize {
			status := fmt.Sprintf("Byte count: %s", bytefmt.ByteSize(n))
			if *dostats {
				fmt.Printf(clearscreen)
				fmt.Println(status)
				for i, m := range masks {
					frequency := "NaN"
					if hits[m] != 0 {
						frequency = bytefmt.ByteSize(n / hits[m])
					}
					fmt.Printf("0x%016x (%02d bits): every %s\n", m, i+1, frequency)
				}
			} else {
				fmt.Printf(clearline)
				fmt.Printf(status)
				fmt.Printf("\r")
			}
			io.ReadFull(f, rbuf)
			k = 0
		}
		roll.Roll(rbuf[k])
		if *dostats {
			s := hash2uint64(roll.Sum(hbuf))
			for _, m := range masks {
				if s&m == m {
					hits[m] += 1
				} else {
					break
				}
			}
		}
		k++
		n++
	}
	duration := time.Since(t)
	fmt.Printf("Rolled %s of data in %v (%s/s).\n",
		bytefmt.ByteSize(n),
		duration,
		bytefmt.ByteSize(n*1e9/uint64(duration)),
	)
}
