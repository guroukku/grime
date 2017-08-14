package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
)

type Header struct {
	License     [100]byte
	Name        [12]byte
	Version     [8]byte
	Timestamp   [42]byte
	FileSize    uint32
	FormatCount uint16
	FileCount   uint16
	Val1       [2]byte 
	Val2       [2]byte 
	Val3       [2]byte 
	Val4       [2]byte 
	Val5       [2]byte 
}

type FormatInfo struct {
	Name  [4]byte
	Count uint16
	Pos   uint16
}

type FileInfo struct {
	Name [12]byte
	_    uint16
	Size uint32
	Addr uint32
	Val1 uint32
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func readNextBytes(file *os.File, number int) []byte {
	bytes := make([]byte, number)

	num, err := file.Read(bytes)
	if num != number {
		fmt.Printf("Ran out of bytes! (wanted: %d, got: %d)\n", number, num)
	}
	check(err)
	return bytes
}

func getBuffer(f *os.File, n int) *bytes.Buffer {
	data := readNextBytes(f, n)
	buffer := bytes.NewBuffer(data)
	return buffer
}

func unpackHeader(f *os.File, hdrSize int) *Header {
	hdr := Header{}
	err := binary.Read(getBuffer(f, hdrSize), binary.LittleEndian, &hdr)
	check(err)
	return &hdr
}

func unpackFormatList(f *os.File, hdr *Header) []*FormatInfo {
	formatList := make([]*FormatInfo, hdr.FormatCount)
	for i := 0; i < int(hdr.FormatCount); i++ {
		aFormat := FormatInfo{}
		err := binary.Read(getBuffer(f, 8), binary.LittleEndian, &aFormat)
		check(err)
		formatList[i] = &aFormat
	}
	return formatList
}

func unpackFileList(f *os.File, hdr *Header) []*FileInfo {
	fileList := make([]*FileInfo, hdr.FileCount)
	for i := 0; i < int(hdr.FileCount); i++ {
		aFile := FileInfo{}
		err := binary.Read(getBuffer(f, 26), binary.LittleEndian, &aFile)
		check(err)
		fileList[i] = &aFile
	}
	return fileList
}

func unpackFiles(f *os.File, hdr *Header, formats []*FormatInfo, files []*FileInfo)  {	
	for i := 0; i < len(formats); i++ {
		workDir := "./" +  string(bytes.Trim(hdr.Name[:12], "x\000")) + "/" +
			string(formats[i].Name[:3]) + "/"
		fmt.Printf("Extracting to %s\n", workDir)
		os.MkdirAll(workDir, os.ModePerm)
		fmt.Printf("File count: %d\n", formats[i].Count)
		for ii := formats[i].Pos; ii < formats[i].Count+formats[i].Pos; ii++ {
			s := workDir + string(bytes.Trim(files[ii].Name[:12], "x\000"))
			//fmt.Printf("Writing file: %s\n", s)
			w, werr := os.Create(s)
			check(werr)
			addr := int64(files[ii].Addr)
			fsize := int(files[ii].Size)
			f.Seek(addr, 0)
			wdata := (readNextBytes(f, fsize))
			_, werr = w.Write(wdata)
			check(werr)
			w.Close()
		}
	}
}

func unpack(f *os.File, hdr *Header) {
	formatList := unpackFormatList(f, hdr)
	fileList := unpackFileList(f, hdr)
	unpackFiles(f, hdr, formatList, fileList)
}

func main() {
	var hdrSize int

	path := os.Args[1]

	f, err := os.Open(path)
	check(err)
	defer f.Close()

	fmt.Printf("%s opened\n", path)

	formatCheck := readNextBytes(f, 1)

	if formatCheck[0] == byte(0x41) {
		fmt.Printf("Valid RSF format found\n")
		hdrSize = 0xb4
	} else if formatCheck[0] == byte(0x6c) {
		log.Fatal("Cannot handle old-style RSF format\n")
	} else {
		log.Fatal("Unknown file format\n")
	}

	f.Seek(0, 0)
	header := unpackHeader(f, hdrSize)

	fmt.Printf("%s\n%s\n%s\n%s\nFilesize: %d\nFormats: %d Files: %d\n", header.License, header.Name,
		header.Version, header.Timestamp, header.FileSize, header.FormatCount, header.FileCount)

	unpack(f, header)
}
