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

func unpackHeader(file *os.File, hdrSize int) *Header {
	hdr := Header{}
	data := readNextBytes(file, hdrSize)
	buffer := bytes.NewBuffer(data)
	err := binary.Read(buffer, binary.LittleEndian, &hdr)
	if err != nil {
		log.Fatal("binary.Read failed", err)
	}

	return &hdr
}

func getFormatInfo(f *os.File) *FormatInfo {
	aFormat := FormatInfo{}
	data := readNextBytes(f, 8)
	buffer := bytes.NewBuffer(data)
	err := binary.Read(buffer, binary.LittleEndian, &aFormat)
	check(err)
	//fmt.Printf("%s - Count: %d Pos: %d\n", aFormat.Name, aFormat.Count, aFormat.Pos)
	return &aFormat

}

func getFileInfo(f *os.File) *FileInfo {
	aFile := FileInfo{}
	data := readNextBytes(f, 26)
	buffer := bytes.NewBuffer(data)
	err := binary.Read(buffer, binary.LittleEndian, &aFile)
	check(err)
	//fmt.Printf("%s - %x - %x %x\n", aFile.Name, aFile.Addr, aFile.Size, aFile.Val1)
	return &aFile
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
		log.Fatal("Cannot handle DOS RSF format. Quitting.\n")
	} else {
		log.Fatal("Unknown RSF format\n")
	}

	f.Seek(0, 0)
	header := unpackHeader(f, hdrSize)

	fmt.Printf("%s\n%s\n%s\n%s\nFilesize: %d\nFormats: %d Files: %d\n", header.License, header.Name,
		header.Version, header.Timestamp, header.FileSize, header.FormatCount, header.FileCount)

	formatList := make([]*FormatInfo, header.FormatCount)
	for i := 0; i < int(header.FormatCount); i++ {
		formatList[i] = getFormatInfo(f)
	}

	fileList := make([]*FileInfo, header.FileCount)
	for i := 0; i < int(header.FileCount); i++ {
		fileList[i] = getFileInfo(f)
	}

	for i := 0; i < len(formatList); i++ {
		workDir := "./" + string(bytes.Trim(header.Name[:12], "x\000")) + "/" +
			string(formatList[i].Name[:3]) + "/"
		fmt.Printf("Extracting to %s\n", workDir)
		os.MkdirAll(workDir, os.ModePerm)
		fmt.Printf("File count: %d\n", formatList[i].Count)
		for ii := formatList[i].Pos; ii < formatList[i].Count+formatList[i].Pos; ii++ {
			s := workDir + string(bytes.Trim(fileList[ii].Name[:12], "x\000"))
			//fmt.Printf("Writing file: %s\n", s)
			w, werr := os.Create(s)
			check(werr)
			addr := int64(fileList[ii].Addr)
			fsize := int(fileList[ii].Size)
			f.Seek(addr, 0)
			wdata := (readNextBytes(f, fsize))
			_, werr = w.Write(wdata)
			check(werr)
			w.Close()
		}
	}
}
