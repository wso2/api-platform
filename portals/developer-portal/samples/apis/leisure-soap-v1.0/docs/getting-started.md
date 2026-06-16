# Getting Started

Call the Leisure Activities API using any SOAP client or `curl`.

## Search for activities

```xml
<soapenv:Envelope
  xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:lei="http://example.com/leisure">
  <soapenv:Header/>
  <soapenv:Body>
    <lei:SearchActivitiesRequest>
      <lei:location>London</lei:location>
      <lei:category>wellness</lei:category>
      <lei:maxPrice>50.00</lei:maxPrice>
    </lei:SearchActivitiesRequest>
  </soapenv:Body>
</soapenv:Envelope>
```

```bash
curl -X POST https://leisure.example.com/services/LeisureService \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://example.com/leisure/searchActivities" \
  --data @search-request.xml
```

## Book an activity

```xml
<soapenv:Envelope
  xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:lei="http://example.com/leisure">
  <soapenv:Header/>
  <soapenv:Body>
    <lei:BookActivityRequest>
      <lei:activityId>ACT-001</lei:activityId>
      <lei:userId>user-123</lei:userId>
      <lei:date>2025-07-15</lei:date>
      <lei:participants>2</lei:participants>
    </lei:BookActivityRequest>
  </soapenv:Body>
</soapenv:Envelope>
```

## WSDL

The full WSDL is available at the endpoint URL with a `?wsdl` query parameter:

```
https://leisure.example.com/services/LeisureService?wsdl
```
