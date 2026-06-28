# git hooks钩子，主要做代码检测等操作
# .PHONY: git.hooks
# git.hooks:
# 	@chmod +x .githooks/* && cp -R .githooks/* .git/hooks/

# 生成.gitkeep文件
.PHONY: git.keep
git.keep:
	@find . -type d -empty -not -path "./.git/*" -exec touch {}/.gitkeep \;
#	@find . -type d -not -empty -not -path "./.git/*" -exec rm -f {}/.gitkeep \;

# 提交代码
.PHONY: git.commit
git.commit:
	@goji

# 生成tag
.PHONY: git.tag
git.tag:
	@$(BUILD_DIR)scripts/git/gen_tag.sh

# git change log 查看至上次打tag的代码变动
.PHONY: git.log
git.log:
	@git log $(LAST_TAG)..HEAD --pretty=format:%s

# 生成CHANGELOG.md,将按照版本号排序
.PHONY: git.changelog
git.changelog:
	$(eval BUILD_TAG := $(shell git describe --abbrev=0))
	@git-chglog -o CHANGELOG/CHANGELOG-$(BUILD_TAG).md