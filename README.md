# Important
The main repository is hosted at [Codeberg](https://codeberg.org/hohlerde/go-sha1-pow).

# Proof of Work (SHA1)
This repository contains a naive implementation of a Proof of Work (POW) using SHA1 hashes in Go.

## What is Proof of Work?
For a given string (prefix), find another string (suffix) so that SHA1(prefix+suffix) returns a SHA1 hash with N leading zeros. N has also named the difficulty.

Note that the suffix string can be a (hexadecimal) number.

### Example
Difficulty: `5`  
Prefix string: `afsgehfdjue76HGZjdjgREWGDGlie`  
Suffix string: `25706d4d3`  
SHA1: `00 00 07 ce 98 67 a5 dc b9 ec 66 2d 72 72 0a 00 47 4b 5e 6e`

Commandline:  
```
/main --prefix=afsgehfdjue76HGZjdjgREWGDGlie --difficulty=5 --debug=false
```

## Performance
This is a naive, not optimized implementation. The GPU is not used at all. 

For the difficulty of 9, the program can find a solution in approximately 30-40 minutes (16 cores, i7 CPU). Times may vary depending on your hardware and the number of goroutines you use.

Using a high number of goroutines results in a considerable overhead, but the probability of finding a solution is still increased.

## Help
Use the `--help` command line parameter to get an overview of all parameters. 
