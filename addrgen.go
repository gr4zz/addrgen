package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

const (
	VERSION                 = "0.1"
	DEFAULT_SEQUENCE_FORMAT = "ID PUBKEY"
	DEFAULT_RANDOM_FORMAT   = "PRIVKEY PUBKEY"
	HELP                    = `Usage:
  addrgen [-sequence] [-format "ID PUBKEY"] PREFIX [START_ID=1] [COUNT=1000]
  addrgen -random [-format "PRIVKEY PUBKEY"] [COUNT=1000]
  addrgen -seed [LENGTH=32]

Options:
  -sequence       the private key is SHA256($PREFIX$ID)
  -random         generate in random
  -format         available fields ID PRIVKEY PUBKEY BASE
  -seed           generate random string
  -version        show version
  -help           show help`
)

func ok(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type BitcoinPair struct {
	*btcutil.WIF
}

func NewBitcoinPair(key []byte) *BitcoinPair {
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), key)
	wif, err := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, true)
	ok(err)
	return &BitcoinPair{WIF: wif}

}

func (b *BitcoinPair) GetPubKey() string {
	pubKeyStr, err := btcutil.NewAddressPubKey(b.SerializePubKey(), &chaincfg.MainNetParams)
	ok(err)
	return pubKeyStr.EncodeAddress()
}

func (b *BitcoinPair) GetPrivKey() string {
	return b.String()
}

func main() {
	help := flag.Bool("help", false, "")
	format := flag.String("format", "", "")
	random := flag.Bool("random", false, "")
	version := flag.Bool("version", false, "")
	seed := flag.Bool("seed", false, "")
	flag.Bool("sequence", true, "")
	flag.Parse()
	args := flag.Args()

	var err error
	var startID, count int64
	var prefix string
	startID = 1
	count = 1000

	n := len(args)
	if *version {
		fmt.Println("addrgen version", VERSION)
		return
	}

	if *seed {
		var length int64 = 32
		if n > 0 {
			length, err = strconv.ParseInt(args[0], 10, 64)
			ok(err)
		}
		var output []byte
		for int64(len(output)) < length {
			var buf = make([]byte, length)
			rand.Read(buf)
			buf = []byte(base64.RawStdEncoding.EncodeToString(buf))
			buf = regexp.MustCompile(`([^0-9a-zA-Z]|[0oOiLIl])`).ReplaceAll(buf, []byte(""))
			output = append(output, buf...)
		}
		fmt.Print(string(output[:length]))
		return
	}

	if *random {
		if *format == "" {
			*format = DEFAULT_RANDOM_FORMAT
		}
		if n > 0 {
			count, err = strconv.ParseInt(args[0], 10, 64)
			ok(err)
		}
	} else {
		if *format == "" {
			*format = DEFAULT_SEQUENCE_FORMAT
		}

		if n == 0 {
			fmt.Println(HELP)
			if *help {
				return
			}
			os.Exit(1)
		}
		if n > 0 {
			prefix = args[0]
		}
		if n > 1 {
			startID, err = strconv.ParseInt(args[1], 10, 64)
			ok(err)
		}
		if n > 2 {
			count, err = strconv.ParseInt(args[2], 10, 64)
			ok(err)
		}
	}

	pattern := regexp.MustCompile("(PUBKEY|PRIVKEY|ID|BASE)")
	mp := int64(runtime.GOMAXPROCS(0))
	rchs := make([]chan string, mp)

	for i := int64(0); i < mp; i++ {
		rchs[i] = make(chan string, 2)
		go func(i int64) {
			for j := i; j < count; j += mp {
				id := startID + j
				var base []byte
				if *random {
					base = make([]byte, 32)
					rand.Read(base)
				} else {
					base = []byte(fmt.Sprintf("%s%d", prefix, id))
				}
				hash := sha256.Sum256(base)
				pair := NewBitcoinPair(hash[:])

				r := pattern.ReplaceAllStringFunc(*format, func(m string) string {
					switch m {
					case "PUBKEY":
						return pair.GetPubKey()
					case "PRIVKEY":
						return pair.GetPrivKey()
					case "ID":
						return strconv.FormatInt(id, 10)
					case "BASE":
						return string(base)
					default:
						return m
					}
				})

				rchs[id%mp] <- r
			}
		}(i)
	}

	for id := startID; id < startID+count; id++ {
		r := <-rchs[id%mp]
		fmt.Println(r)
	}
}
