---
title: The sqlrs engine API v0.1.0
language_tabs:
  - shell: Shell
  - http: HTTP
  - javascript: JavaScript
  - ruby: Ruby
  - python: Python
  - php: PHP
  - java: Java
  - go: Go
toc_footers: []
includes: []
search: true
highlight_theme: darkula
headingLevel: 2

---

<!-- Generator: Widdershins v4.0.1 -->

<h1 id="the-sqlrs-engine-api">The sqlrs engine API v0.1.0</h1>

> Scroll down for code samples, example requests and responses. Select a language for code samples from the tabs above or the mobile navigation menu.

Local sqlrs engine HTTP API (MVP).
Only the implemented endpoints are documented here.
Do not edit the file directly; it is generated from the codebase.

Base URLs:

* <a href="http://127.0.0.1:{port}">http://127.0.0.1:{port}</a>

    * **port** -  Default: 8080

License: <a href="https://www.apache.org/licenses/LICENSE-2.0.html">Apache-2.0</a>

# Authentication

- HTTP Authentication, scheme: bearer 

<h1 id="the-sqlrs-engine-api-health">health</h1>

## getHealth

<a id="opIdgetHealth"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/health \
  -H 'Accept: application/json'

```

```http
GET http://127.0.0.1:{port}/v1/health HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://127.0.0.1:{port}/v1/health',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/health',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json'
}

r = requests.get('http://127.0.0.1:{port}/v1/health', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/health', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/health");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/health", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/health`

*Engine health check*

Returns engine status. No auth required.

> Example responses

> 200 Response

```json
{
  "ok": true,
  "version": "dev",
  "instanceId": "9f4d2d4b6c1a4a4ea2d39d1f7b0d8a21",
  "pid": 12345
}
```

<h3 id="gethealth-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[HealthResponse](#schemahealthresponse)|
|405|[Method Not Allowed](https://tools.ietf.org/html/rfc7231#section-6.5.5)|Method not allowed|None|

<aside class="success">
This operation does not require authentication
</aside>

<h1 id="the-sqlrs-engine-api-names">names</h1>

## listNames

<a id="opIdlistNames"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/names \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

```http
GET http://127.0.0.1:{port}/v1/names HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('http://127.0.0.1:{port}/v1/names',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/names',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('http://127.0.0.1:{port}/v1/names', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
    'Authorization' => 'Bearer {access-token}',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/names', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/names");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/names", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/names`

*List names*

Returns name bindings.

<h3 id="listnames-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|instance|query|string|false|Filter by instance id.|
|state|query|string|false|Filter by state id.|
|image|query|string|false|Filter by base image id.|

> Example responses

> 200 Response

```json
[
  {
    "name": "string",
    "instance_id": "string",
    "image_id": "string",
    "state_id": "string",
    "state_fingerprint": "string",
    "status": "active",
    "last_used_at": "2019-08-24T14:15:22Z"
  }
]
```

<h3 id="listnames-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
bearerAuth
</aside>

## getName

<a id="opIdgetName"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/names/{name} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

```http
GET http://127.0.0.1:{port}/v1/names/{name} HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('http://127.0.0.1:{port}/v1/names/{name}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/names/{name}',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('http://127.0.0.1:{port}/v1/names/{name}', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
    'Authorization' => 'Bearer {access-token}',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/names/{name}', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/names/{name}");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/names/{name}", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/names/{name}`

*Get a name binding*

Returns a single name binding.

<h3 id="getname-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|name|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "name": "string",
  "instance_id": "string",
  "image_id": "string",
  "state_id": "string",
  "state_fingerprint": "string",
  "status": "active",
  "last_used_at": "2019-08-24T14:15:22Z"
}
```

<h3 id="getname-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[NameEntry](#schemanameentry)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
bearerAuth
</aside>

<h1 id="the-sqlrs-engine-api-instances">instances</h1>

## listInstances

<a id="opIdlistInstances"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/instances \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

```http
GET http://127.0.0.1:{port}/v1/instances HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('http://127.0.0.1:{port}/v1/instances',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/instances',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('http://127.0.0.1:{port}/v1/instances', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
    'Authorization' => 'Bearer {access-token}',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/instances', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/instances");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/instances", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/instances`

*List instances*

Returns instances.

<h3 id="listinstances-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|state|query|string|false|Filter by state id.|
|image|query|string|false|Filter by base image id.|

> Example responses

> 200 Response

```json
[
  {
    "instance_id": "string",
    "image_id": "string",
    "state_id": "string",
    "name": "string",
    "created_at": "2019-08-24T14:15:22Z",
    "expires_at": "2019-08-24T14:15:22Z",
    "status": "active"
  }
]
```

<h3 id="listinstances-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
bearerAuth
</aside>

## getInstance

<a id="opIdgetInstance"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/instances/{instanceId} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

```http
GET http://127.0.0.1:{port}/v1/instances/{instanceId} HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('http://127.0.0.1:{port}/v1/instances/{instanceId}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/instances/{instanceId}',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('http://127.0.0.1:{port}/v1/instances/{instanceId}', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
    'Authorization' => 'Bearer {access-token}',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/instances/{instanceId}', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/instances/{instanceId}");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/instances/{instanceId}", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/instances/{instanceId}`

*Get an instance*

Returns a single instance by id. If the path segment does not match the
instance id format, it is treated as a name alias. If it matches the id
format, the engine first attempts id lookup, then falls back to name.
When resolved by name, the response is a temporary redirect to the
canonical id-based URL.

<h3 id="getinstance-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|instanceId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "instance_id": "string",
  "image_id": "string",
  "state_id": "string",
  "name": "string",
  "created_at": "2019-08-24T14:15:22Z",
  "expires_at": "2019-08-24T14:15:22Z",
  "status": "active"
}
```

<h3 id="getinstance-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[InstanceEntry](#schemainstanceentry)|
|307|[Temporary Redirect](https://tools.ietf.org/html/rfc7231#section-6.4.7)|Temporary redirect to canonical instance id URL|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|None|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|307|Location|string||Canonical instance URL.|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
bearerAuth
</aside>

<h1 id="the-sqlrs-engine-api-states">states</h1>

## listStates

<a id="opIdlistStates"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/states \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

```http
GET http://127.0.0.1:{port}/v1/states HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('http://127.0.0.1:{port}/v1/states',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/states',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('http://127.0.0.1:{port}/v1/states', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
    'Authorization' => 'Bearer {access-token}',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/states', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/states");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/states", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/states`

*List states*

Returns states.

<h3 id="liststates-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|kind|query|string|false|Filter by prepare kind.|
|image|query|string|false|Filter by base image id.|

> Example responses

> 200 Response

```json
[
  {
    "state_id": "string",
    "image_id": "string",
    "prepare_kind": "string",
    "prepare_args_normalized": "string",
    "created_at": "2019-08-24T14:15:22Z",
    "size_bytes": 0,
    "refcount": 0
  }
]
```

<h3 id="liststates-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
bearerAuth
</aside>

## getState

<a id="opIdgetState"></a>

> Code samples

```shell
# You can also use wget
curl -X GET http://127.0.0.1:{port}/v1/states/{stateId} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

```http
GET http://127.0.0.1:{port}/v1/states/{stateId} HTTP/1.1
Host: 127.0.0.1
Accept: application/json

```

```javascript

const headers = {
  'Accept':'application/json',
  'Authorization':'Bearer {access-token}'
};

fetch('http://127.0.0.1:{port}/v1/states/{stateId}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

```ruby
require 'rest-client'
require 'json'

headers = {
  'Accept' => 'application/json',
  'Authorization' => 'Bearer {access-token}'
}

result = RestClient.get 'http://127.0.0.1:{port}/v1/states/{stateId}',
  params: {
  }, headers: headers

p JSON.parse(result)

```

```python
import requests
headers = {
  'Accept': 'application/json',
  'Authorization': 'Bearer {access-token}'
}

r = requests.get('http://127.0.0.1:{port}/v1/states/{stateId}', headers = headers)

print(r.json())

```

```php
<?php

require 'vendor/autoload.php';

$headers = array(
    'Accept' => 'application/json',
    'Authorization' => 'Bearer {access-token}',
);

$client = new \GuzzleHttp\Client();

// Define array of request body.
$request_body = array();

try {
    $response = $client->request('GET','http://127.0.0.1:{port}/v1/states/{stateId}', array(
        'headers' => $headers,
        'json' => $request_body,
       )
    );
    print_r($response->getBody()->getContents());
 }
 catch (\GuzzleHttp\Exception\BadResponseException $e) {
    // handle exception or api errors.
    print_r($e->getMessage());
 }

 // ...

```

```java
URL obj = new URL("http://127.0.0.1:{port}/v1/states/{stateId}");
HttpURLConnection con = (HttpURLConnection) obj.openConnection();
con.setRequestMethod("GET");
int responseCode = con.getResponseCode();
BufferedReader in = new BufferedReader(
    new InputStreamReader(con.getInputStream()));
String inputLine;
StringBuffer response = new StringBuffer();
while ((inputLine = in.readLine()) != null) {
    response.append(inputLine);
}
in.close();
System.out.println(response.toString());

```

```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        "Authorization": []string{"Bearer {access-token}"},
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://127.0.0.1:{port}/v1/states/{stateId}", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```

`GET /v1/states/{stateId}`

*Get a state*

Returns a single state.

<h3 id="getstate-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|stateId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "state_id": "string",
  "image_id": "string",
  "prepare_kind": "string",
  "prepare_args_normalized": "string",
  "created_at": "2019-08-24T14:15:22Z",
  "size_bytes": 0,
  "refcount": 0
}
```

<h3 id="getstate-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[StateEntry](#schemastateentry)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
bearerAuth
</aside>

# Schemas

<h2 id="tocS_HealthResponse">HealthResponse</h2>
<!-- backwards compatibility -->
<a id="schemahealthresponse"></a>
<a id="schema_HealthResponse"></a>
<a id="tocShealthresponse"></a>
<a id="tocshealthresponse"></a>

```json
{
  "ok": true,
  "version": "dev",
  "instanceId": "9f4d2d4b6c1a4a4ea2d39d1f7b0d8a21",
  "pid": 12345
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|ok|boolean|true|none|True if the engine is healthy.|
|version|string|true|none|Engine version string.|
|instanceId|string|true|none|Unique engine instance identifier.|
|pid|integer(int32)|true|none|Engine process id.|

<h2 id="tocS_NameEntry">NameEntry</h2>
<!-- backwards compatibility -->
<a id="schemanameentry"></a>
<a id="schema_NameEntry"></a>
<a id="tocSnameentry"></a>
<a id="tocsnameentry"></a>

```json
{
  "name": "string",
  "instance_id": "string",
  "image_id": "string",
  "state_id": "string",
  "state_fingerprint": "string",
  "status": "active",
  "last_used_at": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|instance_id|string,null|false|none|none|
|image_id|string|true|none|none|
|state_id|string|true|none|none|
|state_fingerprint|string|false|none|none|
|status|string|true|none|none|
|last_used_at|any|false|none|none|

anyOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string(date-time)|false|none|none|

or

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|missing|
|status|expired|

<h2 id="tocS_InstanceEntry">InstanceEntry</h2>
<!-- backwards compatibility -->
<a id="schemainstanceentry"></a>
<a id="schema_InstanceEntry"></a>
<a id="tocSinstanceentry"></a>
<a id="tocsinstanceentry"></a>

```json
{
  "instance_id": "string",
  "image_id": "string",
  "state_id": "string",
  "name": "string",
  "created_at": "2019-08-24T14:15:22Z",
  "expires_at": "2019-08-24T14:15:22Z",
  "status": "active"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|instance_id|string|true|none|none|
|image_id|string|true|none|none|
|state_id|string|true|none|none|
|name|string,null|false|none|none|
|created_at|string(date-time)|true|none|none|
|expires_at|any|false|none|none|

anyOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string(date-time)|false|none|none|

or

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|null|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|expired|
|status|orphaned|

<h2 id="tocS_StateEntry">StateEntry</h2>
<!-- backwards compatibility -->
<a id="schemastateentry"></a>
<a id="schema_StateEntry"></a>
<a id="tocSstateentry"></a>
<a id="tocsstateentry"></a>

```json
{
  "state_id": "string",
  "image_id": "string",
  "prepare_kind": "string",
  "prepare_args_normalized": "string",
  "created_at": "2019-08-24T14:15:22Z",
  "size_bytes": 0,
  "refcount": 0
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|state_id|string|true|none|none|
|image_id|string|true|none|none|
|prepare_kind|string|true|none|none|
|prepare_args_normalized|string|true|none|none|
|created_at|string(date-time)|true|none|none|
|size_bytes|integer(int64)|false|none|none|
|refcount|integer(int32)|true|none|none|

