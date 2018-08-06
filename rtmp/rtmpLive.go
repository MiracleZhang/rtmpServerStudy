package rtmp

import (
	"container/list"
	"fmt"
	"time"
	//"context"
	//"github.com/MiracleZhang/rtmpServerStudy/amf"
	"github.com/MiracleZhang/rtmpServerStudy/av"
	//"github.com/MiracleZhang/rtmpServerStudy/flv"
	"github.com/MiracleZhang/rtmpServerStudy/flv/flvio"
	"github.com/MiracleZhang/rtmpServerStudy/timer"
	"github.com/MiracleZhang/rtmpServerStudy/log"
)

func (self *Session)rtmpClosePublishingSession(){
	RtmpSessionDel(self)
	//cancel all play
	if self.context != nil {
		self.cancel()
		self.context = nil
	}
	self.isClosed = true
	var next *list.Element
	if self.CursorList != nil {
		CursorList := self.CursorList.GetList()
		self.ReadRegister()
		//free play session
		for e := CursorList.Front(); e != nil; {
			switch value1 := e.Value.(type) {
			case *Session:
				cursorSession := value1

				//so the play no block
				close(cursorSession.PacketAck)
				next = e.Next()
				CursorList.Remove(e)
				e = next
			}
		}
	}
	//close other thing
	//recode

	if self.IsSelf == true {
		RecordPublishDoneHandler(self)
	}
	//hls
	//flv
	//other things
	self.CursorList = nil
	if self.QuicOn == true{
		self.QuicConn.Close()
	}else{
		self.netconn.Close()
	}

}

func (self *Session) rtmpCloseSessionHanler() {
	self.stage++
	if self.publishing == true {
		self.rtmpClosePublishingSession()
	}else{
		self.rtmpClosePlaySession()
	}

}

func (self *Session)rtmpSendMeta()(err error){

	if err = self.writeDataMsg(5, self.avmsgsid, "onMetaData", self.metaData); err != nil {
		return
	}
	self.metaversion = self.pubSession.metaversion
	return
}

func (self *Session) rtmpSendHead() (err error) {

	var streams []av.CodecData

	if self.aCodec == nil && self.vCodec == nil {
		return
	}
	if self.aCodec != nil {
		streams = append(streams, self.aCodec)
	}
	if self.vCodec != nil {
		streams = append(streams, self.vCodec)
	}

	if err = self.rtmpSendMeta();err != nil{
		return
	}

	for _, stream := range streams {
		var ok bool
		var tag *flvio.Tag
		if tag, ok, err = self.CodecDataToTag(stream); err != nil {
			return
		}

		if ok {
			if err = self.writeAVTag(tag, 0); err != nil {
				return
			}
			if err = self.flushWrite(); err != nil {
				return
			}
		}
	}
	//panic(55)
	return
}

func (self *Session) rtmpSendGop() (err error) {

	if self.GopCache == nil {
		return
	}
	for pkt := self.GopCache.RingBufferGet(); pkt != nil; {
		err = self.writeAVPacket(pkt)
		if err != nil {
			self.GopCache = nil
			return err
		}
		if err = self.flushWrite(); err != nil {
			return
		}
		pkt = self.GopCache.RingBufferGet();
	}
	self.GopCache = nil
	return
}

func (self *Session) RtmpSendAvPackets() (err error) {
	for {
		if self.metaversion != self.pubSession.metaversion{
			self.rtmpSendMeta()
		}
		pkt := self.CurQue.RingBufferGet()

		if self.context != nil  {
			select {
			case <-self.context.Done():
			// here publish may over so play is over
				fmt.Println("the publisher is close")
				self.isClosed = true
				return
			default:
			// 没有结束 ... 执行 ...
			}
		}

		if pkt == nil && self.isClosed  != true {
			//t := timer.GlobalTimerPool.Get(time.Second * MAXREADTIMEOUT)
			select {
			case <-self.PacketAck:
			//case <-t.C:
			}
			//timer.GlobalTimerPool.Put(t)
		}

		if self.pubSession.isClosed == true && pkt == nil{
			self.isClosed = true
			fmt.Println("the publisher is close")
			err = fmt.Errorf("%s","Rtmp.PubSession.Closed.And.pkts.Is.Nil")
			return
		}

		if pkt != nil {
			if err = self.writeAVPacket(pkt); err != nil {
				return
			}
		}
	}
}

func (self *Session) ServerSession(stage int) (err error) {
	playTimes:=0
	for self.stage <= stage {
		switch self.stage {
		//first handshake
		case stageHandshakeStart:
			log.Log.Info(self.LogFormat()+"handshake start")
			if err = self.handshakeServer(); err != nil {
				self.netconn.Close()
				return
			}
			log.Log.Info(self.LogFormat() + "rtmp handshake done")
		case stageHandshakeDone:
			log.Log.Info(self.LogFormat() + "rtmp cmd Msg Cycle")
			if err = self.rtmpReadCmdMsgCycle(); err != nil {
				self.netconn.Close()
				return
			}
			log.Log.Info(self.LogFormat() + "rtmp cmd msg cycle done")
		case stageCommandDone:
			if self.publishing {
				//just 推流
				log.Log.Info(self.LogFormat() + "rtmp client is publishing client addr:" + self.RemoteAddr)
				//only publish and relay need cache gop
				err = self.rtmpReadMsgCycle()
				log.Log.Info(self.LogFormat() + "rtmp publish client read msg cycle err:" + err.Error())
				self.stage = stageSessionDone
				continue
			} else if self.playing {
				pubSession:= RtmpSessionGet(self.StreamAnchor)
				if pubSession != nil {
					//register play to the publish

					t := timer.GlobalTimerPool.Get(time.Second * MAXREADTIMEOUT)
					select {
					case pubSession.RegisterChannel <- self:
					case <-t.C:
					//may be is err
					}
					timer.GlobalTimerPool.Put(t)

					self.pubSession = pubSession
					//copy gop,codec here all new play Competitive the publishing lock
					pubSession.RLock()
					self.updatedGop = true
					self.aCodec = pubSession.aCodec
					self.vCodecData = pubSession.vCodecData
					self.aCodecData = pubSession.aCodecData
					self.vCodec = pubSession.vCodec
					self.metaData = pubSession.metaData
					//copy all gop just ptr copy
					self.GopCache = pubSession.GopCache.GopCopy()
					pubSession.RUnlock()

					self.context, self.cancel = pubSession.context, pubSession.cancel
					//send audio,video head and meta
					if err = self.rtmpSendHead(); err != nil {
						self.isClosed = true
						return err
					}
					//send gop for first screen
					if err = self.rtmpSendGop(); err != nil {
						self.isClosed = true
						return err
					}
					err = self.RtmpSendAvPackets()
					self.isClosed = true
					self.stage = stageSessionDone
				} else {
					if noSelf := self.RtmpCheckStreamIsSelf();noSelf != true {
						url1:= "rtmp://" + self.pushIp + "/" + self.App +"?" + "vhost=" + self.Vhost + "/" + self.StreamId +"?relay=1"
						fmt.Println(url1)
						RtmpRelay("tcp", self.pushIp,self.Vhost,self.App,self.StreamId,url1,stageSessionDone)
						time.Sleep(1*time.Second)
						playTimes++
						if playTimes == 5 {
							self.stage++
						}
					}else{
						self.stage++
					}
					continue
				}
			}
		case stageSessionDone:
			//some thing close handler
			self.rtmpCloseSessionHanler()
		}
	}
	return
}