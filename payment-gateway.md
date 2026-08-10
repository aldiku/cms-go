# Payment Gateway

## Concept
Payment Gateway Sistem has crud feature : create payment gateway, update payment gateway, delete payment gateway.

each payment gateway have workflow on it from extracting hook from transaction -> modification payload -> create request to payment gateway -> send request to payment gateway -> receive response from payment gateway -> handle response from payment gateway -> update transaction hook-> do some other function -> save payment gateway log

i want the concept is like workflow builder or node based programming. The node will be like :
Trigger & Action

trigger example : hook transaction received
action example : modify payload, send request to payment gateway, handle response from payment gateway, handle condition, value match, if condition match, update transaction, update payment gateway log, do some other function

modify the api builder to have this feature (workflow builder), when create an API for webhook/hook transaction, we can set the hook on api builder and it will be used to any workflow. i this wi should move the register hook from hardcoded to db for easy modification


# workflow Builder
## API Builder (Endpoint)

example : api builder for receiving hook from payment gateway
endpoint will be /api/hook/payment-gateway/{payment_gateway_id}
we can create multiple endpoint for multiple payment gateway

## workflow Webhook

example workflow for payment gateway

Trigger : hook payment gateway received

Actions : 
- modify payload
- handle response from payment gateway
- update transaction
- update payment gateway log
- do some other function like (trigger notification, send email, etc)
