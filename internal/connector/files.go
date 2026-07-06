package connector

import (
	"context"
	"os"
	"path/filepath"

	"github.com/schmorrison/goshpanel/internal/store"
)

func readSingleFile(ctx context.Context, conn store.ServiceConnector, container, hostPath, containerPath, localPath string, df *dockerFactory) (string, error) {
	if conn.Mode == string(ModeDocker) && container != "" {
		if hostPath != "" {
			b, err := os.ReadFile(hostPath)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
		cli, err := df.For(conn)
		if err != nil {
			return "", err
		}
		return cli.Exec(ctx, container, "cat", containerPath)
	}
	b, err := os.ReadFile(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func writeSingleFile(ctx context.Context, conn store.ServiceConnector, container, hostPath, containerPath, localPath, content string, df *dockerFactory) error {
	if conn.Mode == string(ModeDocker) && container != "" {
		if hostPath != "" {
			if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
				return err
			}
			return os.WriteFile(hostPath, []byte(content), 0o644)
		}
		cli, err := df.For(conn)
		if err != nil {
			return err
		}
		tmp := filepath.Join(os.TempDir(), "goshpanel-config")
		if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
			return err
		}
		defer os.Remove(tmp)
		dest := container + ":" + containerPath
		return cli.Copy(ctx, tmp, dest)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(localPath, []byte(content), 0o644)
}
