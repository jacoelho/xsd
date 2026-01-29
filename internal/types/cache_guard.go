package types

import "sync"

var typeCacheMu sync.RWMutex
var typeCacheCond = sync.NewCond(&typeCacheMu)
