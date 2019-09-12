package VoiceRecognition

import (
	"bytes"
	"encoding/binary"
	"math"
)

//there is no built in type conversion for int16 slice to bytes slice
//a int16 contains two bytes. the two bytes are extraced from the slice and are appended in order to the byte slice
func int16SliceToByteSlice(int16Slice []int16) ([]byte, error) {
	//this might index out of bounds not sure will need to test
	//might need to minus 1 to len
	var byteSlice []byte
	for i := 0; i < len(int16Slice); i++ {
		buf := new(bytes.Buffer)
		err := binary.Write(buf, binary.LittleEndian, int16Slice[i])
		if err != nil {
			return nil, err
		}
		bytes := buf.Bytes()
		byteSlice = append(byteSlice, bytes[0])
		byteSlice = append(byteSlice, bytes[1])
	}
	return byteSlice, nil
}

//there is no built in type conversion from byte slice to int16 slice
//reads two bytes at a time to get a full int16. This process is repeated until the end of the array.
func byteSliceToInt16Slice(byteSlice []byte) ([]int16, error) {
	//this might index out of bounds not sure will need to test
	//might need to minus 1 to len
	var int16Slice []int16
	for i := 0; i < len(byteSlice); i += 2 {
		var sample int16
		buf := bytes.NewReader(byteSlice[i:(i + 2)])
		err := binary.Read(buf, binary.LittleEndian, &sample)
		if err != nil {
			return nil, err
		}
		int16Slice = append(int16Slice, sample)
	}
	return int16Slice, nil
}

//simple takes a mono sample and create a copy of it and places it next to the original in the slice
//pcm is read in order of audio channels. if there are two audio channels it will read a int16 for first channel
//then will read the next int16 for the second channel and so on.
func convertMonoToStero(pcm []int16) []int16 {
	var stereoPCM []int16
	for i := 0; i < len(pcm); i++ {
		sample := pcm[i]
		stereoPCM = append(stereoPCM, sample)
		stereoPCM = append(stereoPCM, sample)
	}
	return stereoPCM
}

func pcmSliceToPCMFrameSlice(pcm []int16) [][]int16 {
	numberOfSamplesNeededFloat := float64(len(pcm)) / float64(frameSizeStereo)
	numberOfSamplesNeededFloat = math.Ceil(numberOfSamplesNeededFloat)
	numberOfSamplesNeeded := int(numberOfSamplesNeededFloat)
	//creating a slice of specific size and slices within that having a specific length
	//the only way todo this is through a loop kinda dumb
	framesPCM := make([][]int16, numberOfSamplesNeeded)
	for i := range framesPCM {
		framesPCM[i] = make([]int16, frameSizeStereo)
	}
	for i := 0; i < len(framesPCM)-1; i++ {
		copy(framesPCM[i], pcm[i*frameSizeStereo:(i+1)*frameSizeStereo+1])
	}
	copy(framesPCM[len(framesPCM)-1], pcm[(numberOfSamplesNeeded-1)*frameSizeStereo:])
	return framesPCM
}

//converts stereo pcm to mono
//removes every second sample. Leaving only one channel.
//this is the worst possible way of converting to mono but also the easiest.
func convertPCMToMono(pcm []int16) []int16 {
	//this only takes audio from one channel instead of merging both channels
	//bad practise but quick and dirty
	var monoPCM []int16
	for i := 0; i < len(pcm); i += 2 {
		sample1 := pcm[i]
		monoPCM = append(monoPCM, sample1)
	}
	return monoPCM
}
