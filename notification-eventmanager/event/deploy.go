package event

import "github.com/weaveworks/flux/event"

/*
{
  "browser": {
    "type": "deploy",
    "text": "Automated release of new image quay.io/weaveworks/scope:master-055a7664.",
    "attachments": [
      {
        "text": "```CONTROLLER                        STATUS   UPDATES\nextra:deployment/demo             success  demo: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nkube-system:deployment/scope-app  success  scope-app: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/collection       success  collection: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/control          success  control: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/pipe             success  pipe: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/query            success  query: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\n```",
        "color": "good",
        "mrkdwn_in": [
          "text"
        ]
      }
    ],
    "timestamp": "2017-10-11T16:43:36.024371476Z"
  },
  "email": {
    "subject": "deploy",
    "body": "\u003cp\u003eAutomated release of new image quay.io/weaveworks/scope:master-055a7664.\n\u003ccode\u003eCONTROLLER                        STATUS   UPDATES\nextra:deployment/demo             success  demo: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nkube-system:deployment/scope-app  success  scope-app: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/collection       success  collection: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/control          success  control: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/pipe             success  pipe: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/query            success  query: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\n\u003c/code\u003e\u003c/p\u003e\n"
  },
  "slack": {
    "username": "fluxy-dev",
    "text": "Automated release of new image quay.io/weaveworks/scope:master-055a7664.",
    "attachments": [
      {
        "text": "```CONTROLLER                        STATUS   UPDATES\nextra:deployment/demo             success  demo: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nkube-system:deployment/scope-app  success  scope-app: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/collection       success  collection: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/control          success  control: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/pipe             success  pipe: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/query            success  query: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\n```",
        "color": "good",
        "mrkdwn_in": [
          "text"
        ]
      }
    ]
  }
}
*/

type DeployData event.ReleaseEventMetadata
