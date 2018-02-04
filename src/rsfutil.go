package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
)

type ByModTime []os.FileInfo

func (fi_s ByModTime) Len() int {
	return len(fi_s)
}

func (fi_s ByModTime) Swap(i, j int) {
	fi_s[i], fi_s[j] = fi_s[j], fi_s[i]
}

func (fi_s ByModTime) Less(i, j int) bool {
	return fi_s[i].ModTime().Before(fi_s[j].ModTime())
}

type RGB struct {
	r uint8
	g uint8
	b uint8
}

type RGBA struct {
	r uint8
	g uint8
	b uint8
	_ uint8
}

type BitmapHeader struct {
	HeaderField	uint16
	Size		uint32
	_		uint32
	DataAddress	uint32
	DIBSize		uint32
	Width		uint32
	Height		uint32
	ColPlanes	uint16
	Bpp		uint16
	_           [24]byte
}

type TextInfo struct {
	Size  uint16
	Start uint32
}

type TextHeader struct {
	ID         uint16
	LineCount uint16
	_          uint16
	EntryCount  uint16
}

//TextHeader IDs:
//
//HELP - 0E
//NPC - 24 to 27
//GAMETEXT - 29
//SUPERID - 2D
//REGO - 35
//CREDITS - 3C
//RACEDESC - 3D
//STORY - 3D
//ID - 4C
//LOGFLAGS - 55
//LOCKHINT - 58
//DICTION - 62
//MASTER - 7E
//SPELLTXT - 01AF
//NPCCLUE - 030F

type Header struct {
	License        [100]byte
	Name           [12]byte
	Version        [8]byte
	Timestamp      [42]byte
	FileSize       uint32
	DirectoryCount uint16
	FileCount      uint16
	Val1           uint16 //unidentified 0x0008
	Val2           uint16 //unidentified 0x001A
	Val3           uint16 //unidentified 0x0006
	Val4           uint16 //unidentified 0x1a64
	Val5           uint16 //unidentified 0xa26b
}

type DirectoryInfo struct {
	Name  [4]byte
	Count uint16
	Addr  uint16
}

type FileInfo struct {
	Name      [12]byte
	ID        uint16 // 0 = Default, 200 = BMP, 1000 = TXT
	Size      uint32
	StartAddr uint32
	EndAddr   uint32
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func insertByte(slice []byte, index int, value byte) []byte {
	s_a := slice[:index+1]
	s_b := slice[index+1:]
	s_a = append(s_a, value)
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

func getPalette(f *os.File, dir_list []*DirectoryInfo, files []*FileInfo, s string) []*RGB {
	for _, dir := range dir_list {
		if string(dir.Name[:3]) == "PAL" {
			fmt.Printf("PAL directory found\n")
			for _, file := range files[dir.Addr : dir.Addr+dir.Count] {
				file_name := string(bytes.Trim(file.Name[:12], "x\000"))
				if file_name == s {
					fmt.Printf("Unpacking palette: %s\n", file_name)
					palette := make([]*RGB, 256)
					f.Seek(int64(file.StartAddr), 0)
					for i := 0; i < 256; i++ {
						pal := readNumBytes(f, 3)
						pal_entry := RGB{
							r: pal[2],
							g: pal[1],
							b: pal[0],
						}
						palette[i] = &pal_entry
					}
					return palette
				}
			}
		}
	}
	log.Fatal("Couldn't find requested PAL file")
	return nil
}

//XOR each text character against its position.
func textShift (t []byte, ti_s []TextInfo) []byte{
	for i := 0; i < len(ti_s); i++ {
		pos := 0
		for ii := 0; ii < int(ti_s[i].Size); ii++ {
			pos = ii + int(ti_s[i].Start)
			t[pos] = t[pos] ^ byte(ii)
		}
	}
	return t
}

func packHeader() {}

func unpackHeader(f *os.File, hdrSize int) *Header {
	hdr := Header{}
	err := binary.Read(getBuffer(f, hdrSize), binary.LittleEndian, &hdr)
	check(err)
	return &hdr
}

func packDirectoryList() {}

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

func packFileList() {}

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

func packFile() {}

func unpackFile(f *os.File, file *FileInfo) []byte {
	addr := int64(file.StartAddr)
	fsize := int(file.Size)
	f.Seek(addr, 0)
	file_data := readNumBytes(f, fsize)
	return file_data
}

func packText(data [][]byte) TextHeader{
	lc := 0
	for i := 0; i < len(data); i++ {
		for ii := 0; ii < len(data[i]); ii++ {
			if data[i][ii] == '\n' {
				data[i][ii] = '\x00'
				lc += 1
			}
		}
	}
	th := TextHeader{
		ID: uint16(0),
		LineCount: uint16(lc),
		EntryCount: uint16(len(data)),
	}
	//ti_s := []TextInfo{}
	return th
}

func unpackText(data []byte) []byte{
	th := TextHeader{}
	ti_s := []TextInfo{}
	err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &th)
	check(err)

	idx := 8
	for i := 0; i < int(th.LineCount); i++ {
		ti := TextInfo{
			Size:  binary.LittleEndian.Uint16(data[idx : idx+2]),
			Start: binary.LittleEndian.Uint32(data[idx+2 : idx+6]),
		}
		idx += 6
		ti_s = append(ti_s, ti)
	}

	//XOR non-header data
	data = textShift(data[idx + int(th.EntryCount * 8):], ti_s)
	for i := 0; i < len(data); i++ {
		if data[i] == '\x00' {
			data[i] = '\n'
		}
	}
	return data
}

func packFiles(p string) {
	wd, err := os.Open(p)
	check(err)
	d_s, err := wd.Readdir(-1)
	check(err)
	wd.Close()
	sort.Sort(ByModTime(d_s))

	var obfs []byte
	//var buf bytes.Buffer
	fname := fmt.Sprintf(p + ".RSF")
	fmt.Printf("Writing to file: %s\n", fname)
	of, err := os.Create(fname)
	check(err)
	defer of.Close()

	for _, d := range d_s {
		wd, err := os.Open(p + string(os.PathSeparator) + d.Name())
		check(err)
		fmt.Printf("Reading directory: %s\n", d.Name())
		f_s, _ := wd.Readdir(-1)
		for _, f := range f_s {
			fmt.Printf("\t%s\n", f.Name())
			file, err := os.Open(p + string(os.PathSeparator) + d.Name() + string(os.PathSeparator) + f.Name())
			check(err)
			err = binary.Read(file, binary.LittleEndian, obfs)
			check(err)
			_, err = of.Write(obfs)
			check(err)
			of.Sync()
			file.Close()
		}
		wd.Close()
	}
}

func unpackFiles(f *os.File, hdr *Header, dir_list []*DirectoryInfo, files []*FileInfo, pal []*RGB) {
	var buf bytes.Buffer
	fmt.Printf("Extracting to:\n")
	for _, dir := range dir_list {
		work_dir := fmt.Sprintf("./%s/%s/", bytes.Trim(hdr.Name[:8], "x\000"), dir.Name[:3])
		fmt.Printf("\t%s\n", work_dir)
		os.MkdirAll(work_dir, os.ModePerm)

		for _, file := range files[dir.Addr : dir.Count+dir.Addr] {
			s := work_dir + string(bytes.Trim(file.Name[:12], "x\000"))
			out, err := os.Create(s)
			check(err)
			out_data := unpackFile(f, file)
			switch file.ID {
			case 0x200: //Bitmap
				dim := out_data[:4]
				bmp_x := uint32(binary.LittleEndian.Uint16(dim[:2]))
				bmp_y := uint32(binary.LittleEndian.Uint16(dim[2:]))
				bmp_data := out_data[4:]
				bmp_header := BitmapHeader{
					HeaderField: 0x4d42,
					Size:        uint32(0x43B + file.Size),
					DataAddress: 0x43B,
					DIBSize:     0x28,
					Width:       bmp_x,
					Height:      bmp_y,
					ColPlanes:   0x1,
					Bpp:         0x8,
				}
				//Some bitmaps are not 4-byte aligned, so we need to check and pad them manually
				row := int(bmp_x)
				rowPad := -(row%4 - 4)
				if rowPad != 4 {
					bmp_data = bmp_data[rowPad:]
					for i := rowPad; i < len(bmp_data); i += row + rowPad {
						for ii := 0; ii < rowPad; ii++ {
							bmp_data = insertByte(bmp_data, i-1, 0)
						}
					}
				}
				binary.Write(&buf, binary.LittleEndian, bmp_header)

				//PAL values are 0x00 - 0x3F so must be multiplied by 4
				for i := 0; i < len(pal); i++ {
					outpal_entry := RGBA{
						r: pal[i].r * 4,
						g: pal[i].g * 4,
						b: pal[i].b * 4,
					}
					binary.Write(&buf, binary.LittleEndian, outpal_entry)
				}
				binary.Write(&buf, binary.LittleEndian, bmp_data)
				bmp_file := make([]byte, buf.Len())
				err = binary.Read(&buf, binary.LittleEndian, bmp_file)
				check(err)
				_, err = out.Write(bmp_file)
				check(err)

			case 0x1000: //TXT file
				out_data := unpackText(out_data)
				_, err = out.Write(out_data)
				check(err)
			case 0:
				_, err = out.Write(out_data)
				check(err)
			default:
				fmt.Printf("Unexpected format: %x\n", file.ID)
				_, err = out.Write(out_data)
				check(err)
			}
			out.Close()
		}
	}
}

var xFlag, cFlag string

func init() {
	flag.StringVar(&xFlag, "x", "", "Extract the provided `archive`")
	//flag.StringVar(&cFlag, "c", "", "Create an .RSF from provided `directory`")
}

func main() {

	flag.Parse()

	var hdrSize int

	if xFlag != "" {
		f, err := os.Open(xFlag)
		check(err)
		defer f.Close()

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
		fmt.Printf("\n%s\n%s\n%s\n%s\n\tFilesize: %d\n\tDirectories: %d Files: %d\n\n", header.License, header.Name,
			header.Version, header.Timestamp, header.FileSize, header.DirectoryCount, header.FileCount)
		directory_list := unpackDirectoryList(f, int(header.DirectoryCount))
		file_list := unpackFileList(f, int(header.FileCount))
		rgb_pal := getPalette(f, directory_list, file_list, "TRUERGB.PAL")
		//l23_pal := getPalette(f, header, format_list, file_list, "L23.PAL")
		unpackFiles(f, header, directory_list, file_list, rgb_pal)

	} else {
		flag.Usage()
	}

	//if cFlag != "" {
	//	packFiles(cFlag)
	//}
}
