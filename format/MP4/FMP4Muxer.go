package MP4

import (
	"github.com/panda-media/muxer-fmp4/format/AVPacket"
	"github.com/panda-media/muxer-fmp4/format/MP4/commonBoxes"
	"bytes"
)

const (
	MEDIA_AV = iota
	MEDIA_Audio_Only
	MEDIA_Video_Only
)

type FMP4Muxer struct {
	audioHeader      *AVPacket.MediaPacket
	videoHeader      *AVPacket.MediaPacket
	sequence_numberA uint32 //1 base
	sequence_numberV uint32 //1 base
	trunAudio *commonBoxes.TRUN
	trunVideo *commonBoxes.TRUN
	media_data *bytes.Buffer
	sidx *commonBoxes.SIDX
	timescale uint32
	timeBeginMS uint32
	timeLastMS uint32
}
