package quematch

type MapInfo struct {
	MapID          uint32 // ID
	MatchTotalNeed int32  // 匹配需求总人数
	MatchSingleMax int32  // 单组需要人数
}

// ClientKey...
type ClientKey struct {
	ServerID uint32
}

// ClientInfo...
type ClientInfo struct {
	CurPlayerNum int32 // 当前多少人
	MaxPlayerNum int32 // 最大多少人
}

// 获取还能承载多少人
func (c *ClientInfo) hungry() int32 {
	return c.MaxPlayerNum - c.CurPlayerNum
}

// 匹配client服务器
type matchClient struct {
	key    ClientKey
	load   ClientInfo // client服务器负载信息
	notUse bool       // 是否暂不可用
}

func (mc *matchClient) canMatch() bool {
	if mc.notUse {
		return false
	}
	// 如果负载不够了
	clientHungry := mc.load.hungry()
	if clientHungry <= 0 {
		return false
	}
	return true
}
