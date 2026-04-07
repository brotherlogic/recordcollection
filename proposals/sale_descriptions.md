# Sale Descriptions

Before we list something for sale we check:

1. That is has both a sleeve and media condition
1. That is has associated notes that are not empty

If either of these things are false, we fail the update completely - we should have a recordalert for missing
condition and notes, so retrying until those issues are addressed is fine.

If both of these are true then list the item for sale, call out to the external sale-description-generator service (see https://github.com/brotherlogic/sale-description-generator for proto details) to get the sale description and add it in the notes field. Fail the request if the call to sale descrition fails in any way.

The sale description service runs in the local network at 192.168.68.157:30050 

Tests should validate that missing conditions or notes fails, and a failure in the sale description service causes the request to fail.

## Implementation

