package dashSlicer

import (
	"errors"
	"fmt"
	"github.com/panda-media/muxer-fmp4/codec/AAC"
	"github.com/panda-media/muxer-fmp4/dashSlicer/AVSlicer"
	"github.com/panda-media/muxer-fmp4/format/AVPacket"
	"github.com/panda-media/muxer-fmp4/format/MP4"
	"github.com/panda-media/muxer-fmp4/mpd"
	"strings"
)

type DASHSlicer struct {
	minSegmentDuration    int
	maxSegmentDuration    int //valid when audio only
	maxSegmentCountInMPD  int
	lastBeginTime         int
	h264Processer         AVSlicer.SlicerH264
	aacProcesser          AVSlicer.SlicerAAC
	audioHeaderMuxed      bool
	videoHeaderMuxed      bool
	videoTimescale int
	muxerV                *MP4.FMP4Muxer //video only
	muxerA                *MP4.FMP4Muxer //audio only
	audioFrameCount       int
	lastAudioTagBeginTime uint32
	mpd                   *mpd.MPDDynamic
	receiver              FMP4Receiver
}

func NEWSlicer(minLengthMS, maxLengthMS, maxSegmentCountInMPD,videoTimescale int, receiver FMP4Receiver) (slicer *DASHSlicer, err error) {
	slicer = &DASHSlicer{}
	slicer.minSegmentDuration = minLengthMS
	slicer.maxSegmentDuration = maxLengthMS
	slicer.maxSegmentCountInMPD = maxSegmentCountInMPD
	slicer.receiver = receiver
	slicer.videoTimescale=videoTimescale
	if maxSegmentCountInMPD < 2 || nil == receiver || maxLengthMS <= 1 ||videoTimescale<1{
		err = errors.New("invalid param ")
		return nil, err
	}
	slicer.init()

	return
}

//add one or more nal
func (this *DASHSlicer) AddH264Nals(data []byte) (err error) {
	tags := this.h264Processer.AddNals(data)
	if tags == nil || tags.Len() == 0 {
		return
	}
	for e := tags.Front(); e != nil; e = e.Next() {
		tag := e.Value.(*AVPacket.MediaPacket)
		err = this.appendH264Tag(tag)
		if err != nil {
			err = errors.New("AddH264Nals failed:" + err.Error())
			return
		}
	}
	return
}

func (this *DASHSlicer) appendH264Tag(tag *AVPacket.MediaPacket) (err error) {
	if this.videoHeaderMuxed == false && tag.Data[0] == 0x17 && tag.Data[1] == 0 {
		err = this.muxerV.SetVideoHeader(tag)
		if err != nil {
			err = errors.New("set video header :" + err.Error())
			return
		}
		this.mpd.SetVideoInfo(this.videoTimescale, this.h264Processer.Width(), this.h264Processer.Height(), this.h264Processer.FPS(),
			1, this.h264Processer.Codec())
		this.videoHeaderMuxed = true
		var videoHeader []byte
		videoHeader, err = this.muxerV.GetInitSegment()
		this.receiver.VideoHeaderGenerated(videoHeader)
		return
	}

	if tag.Data[0] == 0x17 && tag.Data[1] == 1 {
		if this.newslice(tag.TimeStamp) {
			_, moofmdat, duration, bitrate, err := this.muxerV.Flush()
			if err != nil {
				return err
			}
			this.mpd.SetVideoBitrate(bitrate)

			var timestamp int64
			var durationMP4 int
			timestamp, durationMP4, err = this.mpd.AddVideoSlice(duration, moofmdat)
			this.receiver.VideoSegmentGenerated(moofmdat, timestamp, durationMP4)
			if this.audioHeaderMuxed {
				_, moofmdat, _, bitrate, er := this.muxerA.Flush()
				if er != nil {
					return er
				}

				this.mpd.SetAudioBitrate(bitrate)

				timestamp, durationMP4, _ := this.mpd.AddAudioSlice(this.audioFrameCount, moofmdat)
				this.receiver.AudioSegmentGenerated(moofmdat, timestamp, durationMP4)
				this.audioFrameCount = 0

			}

		}
	}
	err = this.muxerV.AddPacket(tag)
	if err != nil {
		return
	}
	return
}

//add one nal

func (this *DASHSlicer) AddH264Frame(nal []byte) (err error) {
	tag := this.h264Processer.AddNal(nal)
	if nil == tag {
		return
	}
	err=this.appendH264Tag(tag)
	if err != nil {
		err = errors.New("AddH264Frame failed:" + err.Error())
		return
	}
	return
}

//add one  aac frame
func (this *DASHSlicer) AddAACFrame(data []byte) (err error) {
	tag := this.aacProcesser.AddFrame(data)
	if tag == nil {
		err = errors.New("invalid aac data")
		return
	}
	if false == this.audioHeaderMuxed {
		this.lastAudioTagBeginTime = tag.TimeStamp
		this.muxerA.SetAudioHeader(tag)
		this.audioHeaderMuxed = true
		this.mpd.SetAudioInfo(this.aacProcesser.SampleRate(),
			this.aacProcesser.SampleRate(),
			16,
			this.aacProcesser.Channel(),
			AAC.SAMPLE_SIZE,
			this.aacProcesser.Codec())
		audioHeader, err := this.muxerA.GetInitSegment()
		if err != nil {
			return err
		}
		this.receiver.AudioHeaderGenerated(audioHeader)
	} else {
		this.muxerA.AddPacket(tag)
		this.audioFrameCount++
		if false == this.videoHeaderMuxed && tag.TimeStamp-this.lastAudioTagBeginTime > uint32(this.maxSegmentDuration) {
			_, moofmdat, _, bitrate, er := this.muxerA.Flush()
			if er != nil {
				return er
			}

			this.mpd.SetAudioBitrate(bitrate)

			timestamp, durationMP4, _ := this.mpd.AddAudioSlice(this.audioFrameCount, moofmdat)
			this.receiver.AudioSegmentGenerated(moofmdat, timestamp, durationMP4)
			this.audioFrameCount = 0
		}
	}
	return
}

//get the last MPD
func (this *DASHSlicer) GetMPD() (data []byte, err error) {
	data, err = this.mpd.GetMPDXML()
	return
}

func (this *DASHSlicer) init() {
	this.muxerV = MP4.NewMP4Muxer()
	this.muxerA = MP4.NewMP4Muxer()
	this.mpd = mpd.NewDynamicMPDCreater(this.minSegmentDuration, this.maxSegmentCountInMPD)
}

func (this *DASHSlicer) newslice(timestamp uint32) bool {
	if int(timestamp)-this.lastBeginTime >= this.minSegmentDuration {
		this.lastBeginTime = int(timestamp)
		return true
	}
	return false
}

func (this *DASHSlicer) GetVideoData(param string) (data []byte, err error) {
	if strings.Contains(param, "_init_") {
		data, err = this.muxerV.GetInitSegment()
	} else {
		id := int64(0)
		fmt.Sscanf(param, "video_video0_%d_mp4.m4s", &id)
		data, err = this.mpd.GetVideoSlice(id)
	}
	return
}

func (this *DASHSlicer) GetAudioData(param string) (data []byte, err error) {
	if strings.Contains(param, "_init_") {
		data, err = this.muxerA.GetInitSegment()
	} else {
		id := int64(0)
		fmt.Sscanf(param, "audio_audio0_%d_mp4.m4s", &id)
		data, err = this.mpd.GetAudioSlice(id)
	}
	return
}

//notice the slicer stream end
func (this *DASHSlicer)EndofStream(){
	if this.videoHeaderMuxed{
		//video only or av
		_, moofmdat, duration, bitrate, err := this.muxerV.Flush()
		if err != nil {
			return
		}
		this.mpd.SetVideoBitrate(bitrate)

		var timestamp int64
		var durationMP4 int
		timestamp, durationMP4, err = this.mpd.AddVideoSlice(duration, moofmdat)
		this.receiver.VideoSegmentGenerated(moofmdat, timestamp, durationMP4)
		if this.audioHeaderMuxed {
			_, moofmdat, _, bitrate, err := this.muxerA.Flush()
			if err != nil {
				return
			}

			this.mpd.SetAudioBitrate(bitrate)

			timestamp, durationMP4, _ := this.mpd.AddAudioSlice(this.audioFrameCount, moofmdat)
			this.receiver.AudioSegmentGenerated(moofmdat, timestamp, durationMP4)
			this.audioFrameCount = 0

		}
	}else if this.audioHeaderMuxed{
		//audio only
		_, moofmdat, _, bitrate, err := this.muxerA.Flush()
		if err != nil {
			return
		}

		this.mpd.SetAudioBitrate(bitrate)

		timestamp, durationMP4, _ := this.mpd.AddAudioSlice(this.audioFrameCount, moofmdat)
		this.receiver.AudioSegmentGenerated(moofmdat, timestamp, durationMP4)
		this.audioFrameCount = 0
	}

}
