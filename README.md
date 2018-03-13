# Copy Autoscaler CLI Plugin
The plugin can be used to import and export autoscaler settings from the CLI

Inspired by https://github.com/Pivotal-Field-Engineering/autoscaling-cli-plugin

### Installation
```bash
cf install-plugin http://stash.corp.web:7990/scm/pcf/copy-autoscaler.git
```

### Usage

```bash
$ cf copy-autoscaler helloworld export autoscaler-settings.json
exporting autoscaler-settings.json for helloworld

done.

$ cat autoscaler-settings.json
{
  "rules": {
    "min_instances": 1,
    "max_instances": 5,
    "enabled": true,
    "relationships": {
      "rules": [
        {
          "guid": "",
          "type": "cpu",
          "enabled": true,
          "sub_type": "",
          "min_threshold": 10,
          "max_threshold": 50
        },
        {
          "guid": "",
          "type": "http_latency",
          "enabled": true,
          "sub_type": "avg_99th",
          "min_threshold": 10,
          "max_threshold": 60
        },
        {
          "guid": "",
          "type": "http_throughput",
          "enabled": false,
          "sub_type": "",
          "min_threshold": 50,
          "max_threshold": 100
        }
      ]
    }
  },
  "schedule": {
    "resources": [
      {
        "executes_at": "2020-04-14T01:02:00Z",
        "min_instances": 1,
        "max_instances": 6,
        "recurrence": 0,
        "enabled": true
      },
      {
        "executes_at": "2021-01-01T00:00:00Z",
        "min_instances": 2,
        "max_instances": 5,
        "recurrence": 64,
        "enabled": true
      }
    ]
  }
}%

$ cf copy-autoscaler helloworld import autoscaler-settings.json
importing autoscaler-settings.json for helloworld

done.
```