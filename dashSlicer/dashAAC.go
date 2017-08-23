package dashSlicer

import (
	"github.com/panda-media/muxer-fmp4/format/AVPacket"
	"github.com/panda-media/muxer-fmp4/codec/AAC"
	"logger"
)

type dashAAC struct {
	headerDecode bool
	asc *AAC.AACAudioSpecificConfig
	frameCount int64
}

func (this *dashAAC)addFrame(data []byte)(tag *AVPacket.MediaPacket)  {
	if data==nil||len(data)==0{
		logger.LOGF(data)
		return
	}
	if false==this.headerDecode{
		this.asc=AAC.AACGetConfig(data)
		if this.asc.ObjectType()==0||this.asc.SampleRate()==0||
		this.asc.Channel()==0{
			return
		}

		logger.LOGD(*this.asc)
		this.headerDecode=true
		tag=&AVPacket.MediaPacket{}
		tag.PacketType=AVPacket.AV_PACKET_TYPE_AUDIO
		tag.TimeStamp=0
		tag.Data=make([]byte,2+len(data))
		tag.Data[0]=0xaf
		tag.Data[1]=0
		copy(tag.Data[2:],data)
	}else{
		tag=&AVPacket.MediaPacket{}
		tag.PacketType=AVPacket.AV_PACKET_TYPE_AUDIO
		tag.Data=make([]byte,2+len(data))
		tag.TimeStamp=this.calNextTimeStamp()
		tag.Data[0]=0xaf
		tag.Data[1]=1
		copy(tag.Data[2:],data)
	}
	return
}

func (this *dashAAC) calNextTimeStamp()(timestamp uint32	){
	this.frameCount++
	timestamp=uint32((this.frameCount*1000*AAC.SAMPLE_SIZE/int64(this.asc.SampleRate()))&0xffffffff)
	return
}