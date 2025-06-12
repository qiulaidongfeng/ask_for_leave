//go:build !make_root

package afl

import "github.com/gin-gonic/gin"

func MakeRoot(s *gin.Engine) {}
