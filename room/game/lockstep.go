package game

type command struct {
	id  uint64
	sid int32
	sx  int32
	sy  int32
}

type frameData struct {
	idx  uint32
	cmds []*command
}

func newFrameData(index uint32) *frameData {
	f := &frameData{
		idx:  index,
		cmds: make([]*command, 0),
	}

	return f
}

type lockstep struct {
	frames     map[uint32]*frameData
	frameCount uint32
}

func newLockstep() *lockstep {
	l := &lockstep{
		frames: make(map[uint32]*frameData),
	}

	return l
}

func (l *lockstep) reset() {
	l.frames = make(map[uint32]*frameData)
	l.frameCount = 0
}

func (l *lockstep) getFrameCount() uint32 {
	return l.frameCount
}

func (l *lockstep) pushCmd(cmd *command) {
	f, ok := l.frames[l.frameCount]
	if !ok {
		f = newFrameData(l.frameCount)
		l.frames[l.frameCount] = f
	}

	f.cmds = append(f.cmds, cmd)

}

func (l *lockstep) tick() uint32 {
	l.frameCount++
	return l.frameCount
}

func (l *lockstep) getRangeFrames(from, to uint32) []*frameData {
	ret := make([]*frameData, 0, 3)

	for ; from <= to && from <= l.frameCount; from++ {
		f, ok := l.frames[from]
		if !ok {
			continue
		}
		ret = append(ret, f)
	}

	return ret
}

func (l *lockstep) getFrame(idx uint32) *frameData {

	return l.frames[idx]
}
