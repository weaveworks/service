#!/bin/sh

set -eu

SRC_PATH=$GOPATH/src/github.com/weaveworks/service

# Mount the scope repo:
#  -v $(pwd):/go/src/github.com/weaveworks/service

# If we run make directly, any files created on the bind mount
# will have awkward ownership.  So we switch to a user with the
# same user and group IDs as source directory.  We have to set a
# few things up so that sudo works without complaining later on.
uid=$(stat --format="%u" "$SRC_PATH")
gid=$(stat --format="%g" "$SRC_PATH")
echo "weave:x:$uid:$gid::$SRC_PATH:/bin/sh" >>/etc/passwd
echo "weave:*:::::::" >>/etc/shadow
echo "weave	ALL=(ALL)	NOPASSWD: ALL" >>/etc/sudoers

# Grant full access to paths where dependencies are typically installed:
chmod -R 777 /go/pkg /usr/local/go/pkg

su weave -c "PATH=$PATH make -C $SRC_PATH BUILD_IN_CONTAINER=false $*"
