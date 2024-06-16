## Get started
Get the required modules for benthos.
Compile the example files
Run the example file. in Windows: 
./example.exe -c benthos.yaml 


### smtp
Smtp output, works with non authenticated servers, TLS and SSL.
Tested against smtp.freesmtpservers.com and hotmail.
Does not support Oauth. Make sure you use some trigger mechanism to not spam the server

```yaml
---
output:
  smtp:
    serverAddress: smtp-mail.outlook.com
    serverPort: 587
    senderAddress: "test@hotmail.com"     # needs to be the same as user unless the server supports relaying
    recipients: ["test@testa.com"]        # add one or more recipients.
    username: "test@hotmail.com"          # only needed the using TLS/SSL
    password: "password"                  # only needed the using TLS/SSL
    TLS: "STARTTLS"                       # options STARTTLS or SMTPS false if omitted 
    InsecureSkipVerify: false             # false if omitted, only used 
  ```

#### benthos pipeline Parameters
The output plugin reads the context in the service message from benthos and extracts 2 string objects: <b>root.msg</b> and <b>root.subject</b>. 
Example use:
```yaml
  root.msg = "error, high level: " + this.value.string()
  root.subject = "test alarm"
```

#### Alarm trigger example using mqtt and cache
In the following example we use mqtt for input.
- First processor reads the mqtt message and extract topic, value and sets a timestamp.
- Second we use a branch to handle the cache in parallell with the normal message handling and try to read the cached message
- Third we use a bloblang processor to set the conditions for the alarm and the trigger. if the value is greater than 50 and the trigger is false then we set a alarm message and the trigger so it only sends 1 message. When the value drops below 10 we reset the trigger and allow it to re-trigger and send a new message
- Forth we set the current trigger value to the cache.

In the output stage we use a case to check if send_email is true and if so we use the smtp output so send an email

Lastly we add the cache instance to allow for caching values in the memory and set the inital value to false.
```yaml
input:
  label: ""
  mqtt:
    urls: [tcp://192.168.0.1:1883]
    client_id: "smtp"
    connect_timeout: 30s
    topics: [test/new/someValue]
    auto_replay_nacks: true
pipeline:
  processors:
    - bloblang: |
        root = {
          "mqtt_topic": meta("mqtt_topic"), 
          "timestamp_ms": (timestamp_unix_nano() / 1000000).floor(),
          "value": this.number()
        }
    - branch:
        request_map: |
          root = {}
        processors:
          - try:
              - cache:
                  operator: get
                  resource: trigger
                  key: 'trigger_set'
        result_map: 'root.trigger_set = content().string()'

    - bloblang: |
        root.value = this.value

        if this.value > 50 && this.trigger_set == "false" {
          root.msg = "error, high level in " + this.mqtt_topic.string() + ": " + this.value.string()
          root.subject = "test alarm"
          root.send_email = "true"
          root.set_trigger = "true"
        } else if this.value < 10 {
          root.send_email = "false"
          root.set_trigger = "false"
        } else {
          root.send_email = "false"
          root.set_trigger = this.trigger_set
        }

    - cache:
        operator: set
        resource: trigger
        key: 'trigger_set'
        value: '${! json("set_trigger") }'
 
output:
  switch:
    cases:
      - check: this.send_email == "true"
        output:
          smtp:
            serverAddress: smtp-mail.outlook.com
            serverPort: 587
            senderAddress: "test@hotmail.com"     # needs to be the same as user unless the server supports relaying
            recipients: ["test@testa.com"]        # add one or more recipients.
            username: "test@hotmail.com"          # only needed the using TLS/SSL
            password: "password"                  # only needed the using TLS/SSL
            TLS: "STARTTLS"                       # options STARTTLS or SMTPS false if omitted 
            InsecureSkipVerify: false             # false if omitted, only used 
      - output:
          stdout: {} # use for debugging
          
cache_resources:
  - label: trigger
    memory:
      default_ttl: 1h # time the cache is stored
      init_values:
        trigger_set: "false"
```