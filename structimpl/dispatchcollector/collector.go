package dispatchcollector

import (
	"xcore/xcontainer/timer"
	"xcore/xlog"
)

/*
	collect.go: 单个collect实现
*/

// 收集器最终的结果
const (
	ECollectRetUnknown = iota // 未出结果
	ECollectRetSuccess        // 成功
	ECollectRetFailed         // 失败
)

// 收集的结果
const (
	ECollectUnknown = iota // 未知
	ECollectAccept         // 接受
	ECollectRefuse         // 拒绝
)

// 上层依赖接口
type dispatchInterface interface {
	OnCollectOne(collID uint32, roleID uint32, accept bool) // 收集一个结果
	OnCollectSuccess(collID uint32)                         // 收集成功
	OnCollectFailed(collID uint32, roleID uint32)           // 收集失败
}

// 一个收集详细信息
type oneCollectDetailInfo struct {
	result int         // 收集结果
	ExData interface{} // 额外数据
}

// 一次分发-收集
type oneCollect struct {
	interf    dispatchInterface                // 收集器业务interface
	collID    uint32                           // ID
	collInfoM map[uint32]*oneCollectDetailInfo // key->ID, value->详细信息
	timeout   *timer.Item                      // 收集超时timer
	ExData    interface{}                      // 收集器额外数据
}

// --------------------------- 内置函数 ---------------------------

// new
func newCollectDetailInfo(exData interface{}) *oneCollectDetailInfo {
	return &oneCollectDetailInfo{
		result: ECollectUnknown,
		ExData: exData,
	}
}

// new
func newOneCollect(collID uint32, timeout *timer.Item, interf dispatchInterface) *oneCollect {
	coll := &oneCollect{
		interf:    interf,
		collID:    collID,
		collInfoM: make(map[uint32]*oneCollectDetailInfo),
		timeout:   timeout,
	}
	return coll
}

// 收集一个结果
// 返回: 收集的结果
func (c *oneCollect) collectOne(key uint32, agree bool) {
	result := ECollectUnknown
	if _, ok := c.collInfoM[key]; !ok {
		return
	}
	if agree {
		result = ECollectAccept
	} else {
		result = ECollectRefuse
	}
	c.collInfoM[key].result = result
	c.interf.OnCollectOne(c.collID, key, agree)
}

// 检查收集是否结束
// 返回: 收集是否结束
func (c *oneCollect) checkCollectOver() bool {
	collFinish := ECollectRetUnknown
	var refuseRoleID uint32
	c.ForeachColl(func(roleID uint32, state int, exData interface{}) bool {
		switch state {
		case ECollectRefuse:
			// 有人拒绝, 直接返回失败
			collFinish = ECollectRetFailed
			refuseRoleID = roleID
			return false
		case ECollectAccept:
			// 接受
			collFinish = ECollectRetSuccess
			return true
		case ECollectUnknown:
			// 未知
			collFinish = ECollectRetUnknown
			return false
		default:
			xlog.Errorf("DispatchCollectMgr CollectOne err, unKnown state=%d of roleID=%d",
				state, roleID)
		}
		return true
	})
	// 根据收集结果执行回调给业务层
	switch collFinish {
	case ECollectRetSuccess:
		// 收集成功
		c.interf.OnCollectSuccess(c.collID)
	case ECollectRetFailed:
		// 收集失败
		c.interf.OnCollectFailed(c.collID, refuseRoleID)
	case ECollectRetUnknown:
		// 未知, 继续等待
		return false
	default:
		xlog.Errorf("DispatchCollectMgr deal collFinish=%d, unKnown.", collFinish)
		return false
	}
	// 收集结束
	return true
}

// --------------------------- 外置函数 ---------------------------

// 获取collID
func (c *oneCollect) GetCollID() uint32 {
	return c.collID
}

// 增加一个收集
func (c *oneCollect) AddOneCollect(key uint32, exData interface{}) {
	if _, ok := c.collInfoM[key]; ok {
		return
	}
	c.collInfoM[key] = newCollectDetailInfo(exData)
}

// 删除一个收集
func (c *oneCollect) DelOneCollect(key uint32) {
	if _, ok := c.collInfoM[key]; ok {
		return
	}
	delete(c.collInfoM, key)
}

// 遍历
func (c *oneCollect) ForeachColl(runFunc func(roleID uint32, state int, exData interface{}) bool) {
	for rID := range c.collInfoM {
		if !runFunc(rID, c.collInfoM[rID].result, c.collInfoM[rID].ExData) {
			return
		}
	}
}

// 设置额外数据
func (c *oneCollect) SetExData(exData interface{}) {
	c.ExData = exData
}

// 获取额外数据
func (c *oneCollect) GetExData() interface{} {
	return c.ExData
}
