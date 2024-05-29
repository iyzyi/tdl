package forwarder

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Recorder struct {
	FromID int64
	msgIDs map[int]struct{}
	file   *os.File
}

func NewRecorder(fromID int64) (r *Recorder, err error) {
	r = &Recorder{
		FromID: fromID,
		msgIDs: make(map[int]struct{}),
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path, error: %v\n", err)
		return
	}
	execDir := filepath.Dir(execPath)

	recordDir := filepath.Join(execDir, "record")
	err = os.MkdirAll(recordDir, os.ModePerm)
	if err != nil {
		fmt.Printf("Failed to create %v, error: %v\n", recordDir, err)
		return
	}

	recordFileName := fmt.Sprintf("%v.txt", fromID)
	recordFilePath := filepath.Join(recordDir, recordFileName)
	r.file, err = os.OpenFile(recordFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("Failed to open %v, error: %v\n", recordFilePath, err)
		return
	}

	reader := bufio.NewReader(r.file)
	var line string
	var num int
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			fmt.Printf("Failed to read the record file, error: %v\n", err)
			return
		}
		num, err = strconv.Atoi(line[:len(line)-1])
		if err != nil {
			fmt.Printf("Failed to parse integer of the record file, error: %v", err)
			return
		}
		r.msgIDs[num] = struct{}{}
	}

	return r, nil
}

func (r *Recorder) Forwarded(msgID int) (err error) {
	line := fmt.Sprintf("%v\n", msgID)
	_, err = r.file.WriteString(line)
	if err != nil {
		fmt.Printf("Failed to write to the record file, error: %v", err)
	}
	return
}

func (r *Recorder) IsForwarded(msgID int) bool {
	_, ok := r.msgIDs[msgID]
	return ok
}
