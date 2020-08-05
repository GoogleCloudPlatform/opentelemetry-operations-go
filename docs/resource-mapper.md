# OpenTelemetry Resource mapper

Turn the resource mapper in the metric exporter into a shared tool. It can be reused for both trace and metric exporters. 

## Motivation

OpenTelemetry Resource labels now can be inserted into trace spans. We want to allow Monitored
Resources to be inserted as well. A shared resource mapper can enable trace exporter to convert
OpenTelemetry Resource to Monitored Resources for insertion. 

## Requirement

For Trace exporter, the monitored resource labels need to be in the form of 
<key = "g.co/r/<monitored_resource_type>/<monitored_resource_key>", value = "<monitored_resource_value>">
However, we need to be aware that users can provide resources that cannot be converted to monitored resources.
As for metric exporter, it does not need the prefix "g.co/r" when converting OT resources to monitored resources. 

### Scheme

We plan to move the resource mapper from metric exporter to tool package for reuse. As for the prefix "g.co/r", we would handle it in trace part. First, we would try to use the resource mapper to convert the resource. If it can be converted, then we would add the prefix "g.co/r" for all relevant key-value pairs. Otherwise, we keep the resource same and inject it into spans as before.
