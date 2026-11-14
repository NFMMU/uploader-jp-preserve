package hooks

import (
	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func CurrentUser(c *gin.Context) (model.User, bool) {
	value, ok := c.Get("currentUser")
	if !ok {
		return model.User{}, false
	}
	user, ok := value.(model.User)
	return user, ok
}
