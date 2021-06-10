package log

import "sigs.k8s.io/controller-runtime/pkg/log"

//GlobalLog  notice can not use in init method
var GlobalLog = log.Log
