package secretmessage

import (
	"context"
	"time"

	"github.com/prometheus/common/log"
)

// Handle secret expiration that flush secrets in DB
func FlushExpiredSecrets(ctl *PublicController, secret *Secret) error {
	for {
		now := time.Now()

		if now.After(secret.ExpiresAt) || now.Equal(secret.ExpiresAt) {
			if err := ctl.db.WithContext(context.TODO()).Unscoped().Where("id = ?", secret.ID).Delete(Secret{}).Error; err != nil {
				log.Errorf("could not flush exipred secret: %v", err)
			}
			log.Infof("secret_id %v successfully flushed", secret.ID)
			break
		}

		time.Sleep(time.Second * 10)
	}

	return nil
}
