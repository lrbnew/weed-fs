package topology

import (
	"storage"
	"errors"
	"fmt"
	"math/rand"
)

type VolumeLayout struct {
	repType         storage.ReplicationType
	vid2location    map[storage.VolumeId]*VolumeLocationList
	writables       []storage.VolumeId // transient array of writable volume id
	pulse           int64
	volumeSizeLimit uint64
}

func NewVolumeLayout(repType storage.ReplicationType, volumeSizeLimit uint64, pulse int64) *VolumeLayout {
	return &VolumeLayout{
		repType:         repType,
		vid2location:    make(map[storage.VolumeId]*VolumeLocationList),
		writables:       *new([]storage.VolumeId),
		pulse:           pulse,
		volumeSizeLimit: volumeSizeLimit,
	}
}

func (vl *VolumeLayout) RegisterVolume(v *storage.VolumeInfo, dn *DataNode) {
	if _, ok := vl.vid2location[v.Id]; !ok {
		vl.vid2location[v.Id] = NewVolumeLocationList()
	}
	if vl.vid2location[v.Id].Add(dn) {
		if len(vl.vid2location[v.Id].list) == v.RepType.GetCopyCount() {
			if vl.isWritable(v) {
				vl.writables = append(vl.writables, v.Id)
			}
		}
	}
}

func (vl *VolumeLayout) isWritable(v *storage.VolumeInfo) bool {
	return uint64(v.Size) < vl.volumeSizeLimit &&
		v.Version == storage.CurrentVersion &&
		!v.ReadOnly
}

func (vl *VolumeLayout) Lookup(vid storage.VolumeId) []*DataNode {
	if location := vl.vid2location[vid]; location != nil {
		return location.list
	}
	return nil
}

func (vl *VolumeLayout) PickForWrite(count int) (*storage.VolumeId, int, *VolumeLocationList, error) {
	len_writers := len(vl.writables)
	if len_writers <= 0 {
		fmt.Println("No more writable volumes!")
		return nil, 0, nil, errors.New("No more writable volumes!")
	}
	vid := vl.writables[rand.Intn(len_writers)]
	locationList := vl.vid2location[vid]
	if locationList != nil {
		return &vid, count, locationList, nil
	}
	return nil, 0, nil, errors.New("Strangely vid " + vid.String() + " is on no machine!")
}

func (vl *VolumeLayout) GetActiveVolumeCount() int {
	return len(vl.writables)
}

func (vl *VolumeLayout) removeFromWritable(vid storage.VolumeId) bool {
	for i, v := range vl.writables {
		if v == vid {
			fmt.Println("Volume", vid, "becomes unwritable")
			vl.writables = append(vl.writables[:i], vl.writables[i+1:]...)
			return true
		}
	}
	return false
}
func (vl *VolumeLayout) setVolumeWritable(vid storage.VolumeId) bool {
	for _, v := range vl.writables {
		if v == vid {
			return false
		}
	}
	fmt.Println("Volume", vid, "becomes writable")
	vl.writables = append(vl.writables, vid)
	return true
}

func (vl *VolumeLayout) SetVolumeUnavailable(dn *DataNode, vid storage.VolumeId) bool {
	if vl.vid2location[vid].Remove(dn) {
		if vl.vid2location[vid].Length() < vl.repType.GetCopyCount() {
			fmt.Println("Volume", vid, "has", vl.vid2location[vid].Length(), "replica, less than required", vl.repType.GetCopyCount())
			return vl.removeFromWritable(vid)
		}
	}
	return false
}
func (vl *VolumeLayout) SetVolumeAvailable(dn *DataNode, vid storage.VolumeId) bool {
	if vl.vid2location[vid].Add(dn) {
		if vl.vid2location[vid].Length() >= vl.repType.GetCopyCount() {
			return vl.setVolumeWritable(vid)
		}
	}
	return false
}

func (vl *VolumeLayout) SetVolumeCapacityFull(vid storage.VolumeId) bool {
	return vl.removeFromWritable(vid)
}

func (vl *VolumeLayout) ToMap() interface{} {
	m := make(map[string]interface{})
	m["replication"] = vl.repType.String()
	m["writables"] = vl.writables
	//m["locations"] = vl.vid2location
	return m
}
