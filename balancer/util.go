package balancer

import (
	"os/exec"
	"strings"
)

// Logger for log
type Logger interface {
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
}

func zone() string {
	out, err := exec.Command("/bin/bash", "-c", "/opt/aws/bin/ec2-metadata -z").Output()
	if err != nil {
		return "unknown"
	}

	kv := strings.Split(string(out[:len(out)-1]), " ")
	if len(kv) != 2 {
		return "unknown"
	}

	return kv[1]
}
