package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
)

type BitmapHeader struct {
	HeaderField uint16
	Size uint32
	_ uint32
	DataAddress uint32
	DIBSize uint32
	Width uint16
	Height uint16
	ColPlanes uint16
	Bpp uint16
}

type Header struct {
	License     [100]byte 
	Name        [12]byte
	Version     [8]byte
	Timestamp   [42]byte
	FileSize    uint32
	DirectoryCount uint16
	FileCount   uint16
	Val1       [2]byte //unidentified
	Val2       [2]byte //unidentified
	Val3       [2]byte //unidentified
	Val4       [2]byte //unidentified
	Val5       [2]byte //unidentified
}

type DirectoryInfo struct {
	Name  [4]byte
	Count uint16
	Pos   uint16
}

type FileInfo struct {
	Name [12]byte
	ID uint16 // 0 = Default, 200 = BMP, 1000 = TXT
	Size uint32
	Addr uint32
	Val1 uint8 //unidentified
	Val2 uint8 //unidentified
	Val3 uint8 //unidentified
	Val4 uint8 //unidentified
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func insertByte(slice []byte, index uint16, value byte) []byte {
	s_a := slice[:index]
	s_b := slice[index:]
	s_a = append(s_a, value, value)
	s_a = append(s_a, s_b...)
	return s_a
}

func readNumBytes(file *os.File, number int) []byte {
	bytes := make([]byte, number)
	num, err := file.Read(bytes)
	if num != number {
		fmt.Printf("Ran out of bytes! (wanted: %d, got: %d)\n", number, num)
	}
	check(err)
	return bytes
}

func getBuffer(f *os.File, n int) *bytes.Buffer {
	data := readNumBytes(f, n)
	buffer := bytes.NewBuffer(data)
	return buffer
}

func unpackHeader(f *os.File, hdrSize int) *Header {
	hdr := Header{}
	err := binary.Read(getBuffer(f, hdrSize), binary.LittleEndian, &hdr)
	check(err)
	return &hdr
}

func unpackDirectoryList(f *os.File, cnt int) []*DirectoryInfo {
	dir_list := make([]*DirectoryInfo, cnt)
	for i := 0; i < cnt; i++ {
		dir := DirectoryInfo{}
		err := binary.Read(getBuffer(f, 8), binary.LittleEndian, &dir)
		check(err)
		dir_list[i] = &dir
	}
	return dir_list
}

func unpackFileList(f *os.File, cnt int) []*FileInfo {
	file_list := make([]*FileInfo, cnt)
	for i := 0; i < cnt; i++ {
		file := FileInfo{}
		err := binary.Read(getBuffer(f, 26), binary.LittleEndian, &file)
		check(err)
		file_list[i] = &file
	}
	return file_list
}

func unpackFile(f *os.File, file *FileInfo) []byte {
	addr := int64(file.Addr)
	fsize := int(file.Size)
	f.Seek(addr, 0)
	file_data := readNumBytes(f, fsize)
	
	return file_data
}

func getPalette(f *os.File, dir_list []*DirectoryInfo, files []*FileInfo, s string) []byte {
	for _, dir := range dir_list {
		if string(dir.Name[:3]) == "PAL" {
			fmt.Printf("PAL directory found\n")
			for _, file := range files[dir.Pos:dir.Pos + dir.Count] {
				file_name := string(bytes.Trim(file.Name[:12], "x\000"))
				if file_name == s {
					fmt.Printf("Unpacking palette: %s\n", file_name)
					pal := unpackFile(f, file)

					//PAL values are 0x00 - 0x3F, and red/blue channels seem to be swapped
					for i := 0; i < len(pal); i++ {
						pal[i] *= 4       
						if (i+1) % 3 == 0 {    
							buf := pal[i]
							pal[i] = pal[i-2]
							pal[i-2] = buf
						}
					}
					return pal
				}
			}
		}
	}
	log.Fatal("Couldn't find requested PAL file")
	return nil
}

func unpackFiles(f *os.File, hdr *Header, dir_list []*DirectoryInfo, files []*FileInfo, pal []byte)  {
	var buf bytes.Buffer
	
	for _, dir := range dir_list {
		work_dir := "./" +  string(bytes.Trim(hdr.Name[:8], "x\000")) + "/" +
			string(dir.Name[:3]) + "/"
		fmt.Printf("Extracting to %s\n", work_dir)
		os.MkdirAll(work_dir, os.ModePerm)
		fmt.Printf("File count: %d\n", dir.Count)


		for _, file := range files[dir.Pos:dir.Count + dir.Pos] {
			s := work_dir + string(bytes.Trim(file.Name[:12], "x\000"))
			out, err := os.Create(s)
			check(err)
	
			out_data := unpackFile(f, file)
			
			//fmt.Printf("Filename: %s\n ID: %x\n Val1: %x\n Val2: %x\n Val3: %x\n Val4: %x\n",
			//	file.Name, file.ID, file.Val1, file.Val2, file.Val3, file.Val4)
			switch file.ID {
			case 0x200: //Bitmap
				dim := out_data[:4]
				bmp_x := binary.LittleEndian.Uint16(dim[:2])
				bmp_y := binary.LittleEndian.Uint16(dim[2:])
				bmp_data := out_data[6:]
				bmp_header := BitmapHeader{
					HeaderField: 0x4d42,
					Size: uint32(0x31D + file.Size),
					DataAddress: 0x31D,
					DIBSize: 0x0c,
					Width: bmp_x,
					Height: bmp_y,
					ColPlanes: 0x1,
					Bpp: 0x8,
				}
				
				//Some bitmaps are not 4-byte aligned, so we need to add the padding manually
				if bmp_x % 4 != 0 {
					fmt.Printf("File %s requires padding\n", file.Name)
					row := uint16(bmp_x)
					for i := row; i < uint16(len(bmp_data)) - row; i += row {
						bmp_data = insertByte(bmp_data, i, 0)
					}		
				}
				
				binary.Write(&buf, binary.LittleEndian, bmp_header)
				buf.Write(pal)
				binary.Write(&buf, binary.LittleEndian, bmp_data)
				
				bmp_file := make([]byte, buf.Len())
				err = binary.Read(&buf, binary.LittleEndian, bmp_file)
				check(err)
				_, err = out.Write(bmp_file)
				check(err)

			default:
				_, err = out.Write(out_data)
				check(err)
			}
			out.Close()
		}
	}
}

func main() {
	var hdrSize int

	if len(os.Args) == 1 {
		fmt.Printf("Usage: rsfunpack FILE\n")
		return
	}
	path := os.Args[1]

	f, err := os.Open(path)
	check(err)
	defer f.Close()

	fmt.Printf("%s opened\n", path)

	formatCheck := readNumBytes(f, 1)

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
		header.Version, header.Timestamp, header.FileSize, header.DirectoryCount, header.FileCount)

	directory_list := unpackDirectoryList(f, int(header.DirectoryCount))
	file_list := unpackFileList(f, int(header.FileCount))

	rgb_pal := getPalette(f, directory_list, file_list, "TRUERGB.PAL")
	//l23_pal := getPalette(f, header, format_list, file_list, "L23.PAL")
	
	unpackFiles(f, header, directory_list, file_list, rgb_pal)
}
