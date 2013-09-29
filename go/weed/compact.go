package main

import (
	"code.google.com/p/weed-fs/go/glog"
	"code.google.com/p/weed-fs/go/storage"
)

func init() {
	cmdCompact.Run = runCompact // break init cycle
	cmdCompact.IsDebug = cmdCompact.Flag.Bool("debug", false, "enable debug mode")
}

var cmdCompact = &Command{
	UsageLine: "compact -dir=/tmp -volumeId=234",
	Short:     "run weed tool compact on volume file if corrupted",
	Long: `Force an compaction to remove deleted files from volume files.
  The compacted .dat file is stored as .cpd file.
  The compacted .idx file is stored as .cpx file.

  `,
}

var (
	compactVolumePath = cmdCompact.Flag.String("dir", "/tmp", "data directory to store files")
	compactVolumeId   = cmdCompact.Flag.Int("volumeId", -1, "a volume id. The volume should already exist in the dir.")
)

func runCompact(cmd *Command, args []string) bool {

	if *compactVolumeId == -1 {
		return false
	}

	vid := storage.VolumeId(*compactVolumeId)
	v, err := storage.NewVolume(*compactVolumePath, vid, storage.CopyNil)
	if err != nil {
		glog.Fatalf("Load Volume [ERROR] %s\n", err)
	}
	if err = v.Compact(); err != nil {
		glog.Fatalf("Compact Volume [ERROR] %s\n", err)
	}

	return true
}
