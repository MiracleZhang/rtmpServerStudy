package ts

import (
	"time"
	"github.com/MiracleZhang/rtmpServerStudy/av"
	"github.com/MiracleZhang/rtmpServerStudy/ts/tsio"
)


//ts have many codec data may be
type Stream struct {
	av.CodecData
	demuxer *Demuxer
	muxer   *Muxer

	pid    uint16
	streamId   uint8
	streamType uint8

	tsw       *tsio.TSWriter
	idx  int

	iskeyframe bool
	pts, dts time.Duration
	data []byte
	datalen int
}