#!/usr/bin/env python

import yaml, sys, os, os.path, string

def lint_replication_controller(filename, rc):

  # template should specify name and version labels
  labels = rc["spec"]["template"]["metadata"]["labels"]
  assert "name" in labels, \
    "%s should define name label for template" % filename
  assert "version" in labels, \
    "%s should define version label for template" % filename

  # the selector labels should equal the template labels
  assert rc["spec"]["selector"] == labels, \
    "%s selector does not match labels" % filename

  # the name of the rc should be name-version
  name = labels["name"]
  version = labels["version"]
  expected = "%s-%s" % (name, version)
  assert rc["metadata"]["name"] == expected, \
    "%s name doesn't match labels (expected: %s)" % (filename, expected)

  # the name label object should match filename
  expected = string.rsplit(os.path.basename(filename), "-", 1)[0]
  assert name == expected, \
    "%s should container an object called %s, not %s" % (filename, expected, name)

  # iff there is one container, the image tag should be the versions
  if len(rc["spec"]["template"]["spec"]["containers"]) == 1:
    container = rc["spec"]["template"]["spec"]["containers"][0]
    image_version = container["image"].split(":", 1)[1]
    assert image_version == version, \
      "%s version label doesn't match image version" % filename

def lint_file(path):
  with open(path, 'r') as stream:
    obj = yaml.load(stream)

  # the top level object (replication controller, service) shouldn't
  # have any labels
  assert "labels" not in obj["metadata"], \
    "%s should not define top-level labels" % path

  if obj["kind"] == "ReplicationController":
    lint_replication_controller(path, obj)

def lint_dir(path):
  for filename in os.listdir(path):
    f = os.path.join(path, filename)
    if os.path.isdir(f):
      lint_dir(f)
    else:
      lint_file(f)

if __name__ == "__main__":
  for path in sys.argv[1:]:
    if os.path.isdir(path):
      lint_dir(path)
    else:
      lint_file(path)

