package service

import (
	"sort"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/samber/lo"
)

// findDeptSubs 找到所有子组织id
// 定义一个函数，用于查找组织的下属组织
func findDeptSubs(depts []*store.DeptModel, nItem *store.DeptModel) (subs []uint64) {
	// 初始化下属组织列表
	subs = make([]uint64, 0)

	// 寻找当前组织直属下级组织
	subItems := lo.Filter(depts, func(item *store.DeptModel, index int) bool {
		return item.ParentID == nItem.ID
	})

	// 如果当前组织没有直属下级组织，则直接返回
	if len(subItems) == 0 {
		return
	}

	// 保存下属组织的ID
	tempSub := make([]uint64, 0)
	lo.ForEach(subItems, func(item *store.DeptModel, index int) {
		tempSub = append(tempSub, item.ID)
	})
	subs = append(subs, tempSub...)

	// 递归查找下属组织的下属组织信息
	lo.ForEach(subItems, func(item *store.DeptModel, index int) {
		subs = append(subs, findDeptSubs(depts, item)...)
	})

	return
}

// deptAssembleTree 把组织组装成组织树
func deptAssembleTree(depts []*store.DeptModel) (trees []store.DeptModel) {
	group := make(map[uint64][]*store.DeptModel)                // 把相同父节点的组织分组
	var treeFunc func(parentid uint64) (tree []store.DeptModel) // 递归处理
	treeFunc = func(parentid uint64) (tree []store.DeptModel) {
		for _, v := range group[parentid] {
			node := *v
			if len(group[v.ID]) > 0 {
				node.Childs = append(node.Childs, treeFunc(node.ID)...)
			}
			tree = append(tree, node)
		}
		sort.Slice(tree, func(i, j int) bool {
			return tree[i].Priority < tree[j].Priority
		})
		return
	}

	for _, v := range depts {
		group[v.ParentID] = append(group[v.ParentID], v)
	}
	return treeFunc(store.DeptModelParentTop)
}

// deptFilterByIDs 根据ids过滤组织
// 只返回包含ids的组织树
func deptFilterByIDs(trees []store.DeptModel, ids map[uint64]struct{}) (ftrees []store.DeptModel) {
	var treeFunc func(tree []store.DeptModel) (ftree []store.DeptModel) // 递归处理
	treeFunc = func(tree []store.DeptModel) (ftree []store.DeptModel) {
		for _, v := range tree {
			ftree = append(ftree, v)
			ftree[len(ftree)-1].Childs = nil

			if len(v.Childs) > 0 {
				ftree[len(ftree)-1].Childs = treeFunc(v.Childs)
			}

			if _, ok := ids[v.ID]; !ok && len(ftree[len(ftree)-1].Childs) == 0 {
				ftree = ftree[:len(ftree)-1]
			}

		}
		return
	}
	return treeFunc(trees)
}

func deptAssembleTreeByRoots(depts []*store.DeptModel, roots []*store.DeptModel) []*store.DeptModel {
	if len(roots) == 0 {
		return nil
	}
	visible := make(map[uint64]*store.DeptModel, len(depts))
	children := make(map[uint64][]*store.DeptModel, len(depts))
	for _, dept := range depts {
		if dept == nil {
			continue
		}
		visible[dept.ID] = dept
		children[dept.ParentID] = append(children[dept.ParentID], dept)
	}
	var build func(*store.DeptModel) *store.DeptModel
	build = func(dept *store.DeptModel) *store.DeptModel {
		if dept == nil {
			return nil
		}
		node := *dept
		node.Childs = nil
		for _, child := range children[dept.ID] {
			if _, ok := visible[child.ID]; !ok {
				continue
			}
			if next := build(child); next != nil {
				node.Childs = append(node.Childs, *next)
			}
		}
		sort.Slice(node.Childs, func(i, j int) bool {
			if node.Childs[i].Priority == node.Childs[j].Priority {
				return node.Childs[i].ID < node.Childs[j].ID
			}
			return node.Childs[i].Priority < node.Childs[j].Priority
		})
		return &node
	}
	out := make([]*store.DeptModel, 0, len(roots))
	for _, root := range roots {
		if next := build(root); next != nil {
			out = append(out, next)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].ID < out[j].ID
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}
