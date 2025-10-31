Promote to production
---------------------

After customizing the content, run “sh compress.sh {nameOfOrg}”, to create a zip file including the customized hbs, image and style files into a zip named after the name of the organization.

Follow the instructions in the [installation guide](https://docs.google.com/document/d/10bIEggNZmHy0oMLGBi_fsszXYYfcztaSt_p2i_3VcPo/edit?pli=1&tab=t.3roll6bfs18k) to setup the database and connect the devportal to it.

**Configure WSO2 as the control plane**

1. Create the IDP for devportal login

**Add claim mappings**  
Go to claims->Add->Add new external claim  
Dialect URI as http://wso2.org/oidc/claim.  
External Clam URI : organizationID  
Mapped Local Claim: http://wso2.org/claims/organization

Go to claims -> List ->  http://wso2.org/oidc/claim.  
Select roles and press edit.  
For the Mapped Local Claim, select http://wso2.org/claims/role

**Add users and roles**

Create super admin role
Got Users and Roles -> Add -> Add new role.  
Give a role name (superAdmin).  
Select all permissions.  
Click finish

Got Users and Roles -> Add -> Add user

Create a new user and assign superAdmin and admin role.  
Create another user with Internal/subscriber permissions.

**Add claims to OIDC scopes**

Goto OIDC scopes->List.  
Click on Add claims for openid.  
Click Add OIDC claim  
Select organizationID and roles as the claim uri


**Add Service Provider**

Goto Service Providers - > Add.  
Give a name and click register.  
Click on claim configuration and select Add claim URI  
Add the following claims

|Service Provider Claim | Local Claim                         | Requested claim  |
|:-------------         |:------------:                       |-------------:    |
| OrganizationID        | http://wso2.org/claims/organization | true             |
| Roles                 | http://wso2.org/claims/role         | true             |
| Username              | http://wso2.org/claims/username     |                  |

Select Username as the Subject Claim URI.

Under inbound authentication configuration, select Oauth and click configure.  
Select code as the grant type.  
Enter the following as the redirect URL:  
regexp=(https://localhost:9443/devportal/services/auth/callback/login|https://localhost:9443/devportal/services/auth/callback/logout|http://localhost:3000/{ORGNAME}/callback|http://localhost:3000/{ORGNAME})

Select PKCE mandatory, Support PKCE 'Plain' Transform Algorithm, Renew Refresh Token and  Allow authentication without the client secret.

Select JWT as the token issuer.

Click update.

**Update the config json**

```bash
 "identityProvider" : {
    "name": "<IDP_Name>",
    "issuer": "<IDP_Issuer>,
    "authorizationURL": "https://<IDP_DOMAIN>/oauth2/authorize",
    "tokenURL": "https://<IDP_DOMAIN>/oauth2/token",
    "userInfoURL": "https://<IDP_DOMAIN>/oauth2/userinfo",
    "clientId": "<Clinet_ID>",
    "callbackURL": "http://localhost:3000/ACME/callback",
    "scope": "openid email groups",
    "signUpURL": "<IDP_SignUP_URL>",
    "logoutURL": "https://<IDP_DOMAIN>/oidc/logout",
    "logoutRedirectURI": "http://localhost:3000/ACME",
    "certificate": "",
    "jwksURL": "https://localhost:9443/oauth2/jwks"
},
"roleClaim": "roles", 
"orgIDClaim": "organizationID",
"groupsClaim": "groups",
"adminRole": "admin",
"subscriberRole": "Interna/subscriber",
"superAdminRole": "superAdmin",
"authenticatedPages": [
    "/*/configure",
    "/portal",
    "/*/applications",
     "/*/applications/*",
     "/*/myAPIs",
     "/*/myAPIs/*"
  ],
"authorizedPages": [
    "/*/applications",
    "/*/applications/*",
    "/*/configure",
    "/portal",
    "/*/myAPIs",
    "/*/myAPIs/*"
  ]
````
API request for creating IDP in developer portal
``` bash
curl --location --request POST 'http://localhost:3000/devportal/organizations/{organizationID}/identityProvider' \
--header 'Content-Type: application/json' \
--header 'Cookie: connect.sid=s%3AhKQhm7b2bCe4RkJuFknvUsxVqgG_iueA.ddy6vv265vp0cRrpRoJMnYZWs11tRTNsT0MKtTyIQ4o' \
--data '{
    "name": "<IDP_Name>",
    "issuer": "<IDP_Issuer>",
    "authorizationURL": "https://<IDP_DOMAIN>/oauth2/authorize",
    "tokenURL": "https://<IDP_DOMAIN>/oauth2/token",
    "userInfoURL": "https://<IDP_DOMAIN>/oauth2/userinfo",
    "clientId": "<Clinet_ID>",
    "callbackURL": "http://localhost:3000/ACME/callback",
    "scope": "openid email groups",
    "signUpURL": "<IDP_SignUP_URL>",
    "logoutURL": "https://<IDP_DOMAIN>/oidc/logout",
    "logoutRedirectURI": "http://localhost:3000/ACME",
    "certificate": "",
    "jwksURL": "https://localhost:9443/oauth2/jwks"
}'
```

2.  Configure APIM CP URL.  

```bash
controlPlane : {
    "url": "https://127.0.0.1:9443/api/am/devportal/v3"
  }
```
3.  Go to APIM publisher and publish an API (https://localhost:9443/publisher)

4.  Store the created API metadata in developer portal side, as mentioned in step 5 of the below section.  


**Follow the steps below to populate the organization’s content in the developer portal.**

1.  Create an organization in the developer portal.

``` bash
curl --location --request POST 'http://localhost:3000/devportal/organizations' 

   --header 'Content-Type: application/json'
   --data-raw '{
    "orgName": "ACME",
    "businessOwner" : "Sachini",
    "businessOwnerContact" : "03434343",
    "businessOwnerEmail" : "sachini@gmail.com",
    "devPortalURLIdentifier": "ACME", //customize URL for the devportal.  
    "roleClaimName":"roles",  
    "groupsClaimName": "groups",  
    "organizationClaimName": "organizationID",
    "organizationIdentifier":"ACME",  
    "adminRole": "admin",
    "subscriberRole": "subscriber", 
    "superAdminRole": "superAdmin"
}'
```

2.  Upload the generated zip with the organization content.
    
``` bash
curl --location --request POST 'http://localhost:3000/devportal/organizations/{orgID}/layout' 
--form 'file=@{pathToZipFile}
```

3. Configure IDP and add IDP details for the organization.

```bash
curl --location 'http://localhost:3000/devportal/organizations//identityProvider' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer <access_token>' \
--data '{
    "name": "IS",
    "issuer": "https://127.0.0.1:9443/oauth2/token",
    "authorizationURL": "https://localhost:9443/oauth2/authorize",
    "tokenURL": "https://127.0.0.1:9443/oauth2/token",
    "userInfoURL": "https://localhost:9443/oauth2/userinfo",
    "clientId": "",
    "callbackURL": "http://localhost:3000/ACME/callback",
    "scope": "openid profile",
    "signUpURL": "",
    "logoutURL": "https://localhost:9443/oidc/logout",
    "logoutRedirectURI": "http://localhost:3000/ACME",
    "certificate": "",
    "jwksURL": "https://localhost:9443/oauth2/jwks"
  }'
```

4.  Navigate to [http://localhost:3000/{orgName}.
    
The organization landing page will include the uploaded content.
The /apis page will render the page with no apis, since no API content is uploaded yet.Follow the steps below to populate the api details in the developer portal.

5.  Create an API in the developer portal:
    
The apiType values include REST, WS, GraphQL or SOAP

This is a multi part request containing a json with metadata related to the API and a file attachement of the api schema definition file.
``` bash
 curl --location 'http://localhost:devportal/organizations/{organizationID}/apis'

   --form 'api-metadata="{
                  "apiInfo": { 
                     "referenceID": "<UUID for the API created in WSO2 publisher>",
                     "apiName": "NavigationAPI",
                     "provider": "WSO2",
                     "orgName": "ACME",
                     "apiCategory": "Travel",
                     "apiDescription": "API for retrieving information about hotels and managing reservations",
                     "visibility" : "PRIVATE",
                     "visibleGroups": \["HR"\],
                     "owners" : {
                       "technicalOwner": "john",
                       "technicalOwnerEmail": "john@gmail.com",
                       "businessOwner" : "sam",
                       "businessOwnerEmail":"sam@gmail.com"
                     },
                     "apiVersion" : "3.0.2",
                     "apiType" : "REST"
                  },
                  "subscriptionPolicies": [
                     {
                        "policyName": "gold"
                     },
                     {
                        "policyName": "platinum"
                     }
                  ],
                  "endPoints": {
                     "sandboxURL": "http://taxi-navigation.mnm.abc.com",
                     "productionURL": "https://taxi-navigation.mnm.abc.com"
                  }
            }"; type=application/json'
   --form 'apiDefinition=@"{apiDefinition.json}"'
```

6. Upload the api landing page content.

To upload the content to be displayed on the api-landing page, create a zip file with the folder structure as follows:
``` bash
{API NAME}
└───content
   │   api-content.hbs
   │   apiContent.md
└───images
   │   icon.svg
   │   product.png
```
This is a multi part request, containing the metadata about the images in the api landing page.   
This should be sent as a json key value map, with the key referring to the name in the hbs where the image is referenced and the name of the image file as the value.  
For example, the key should be sent as api-icon to display the relevant image in the following img tag.
``` bash
    <img src={{apiInfo.apiImageMetadata.api-icon}} class="api-card-img" alt="..." />

```
Run the following command to upload the content.
``` bash
curl --location --request POST 'http://localhost:3000/devportal/organizations/{organizationID}/apis/{apiID}/template'
     --form 'apiContent=@"{path-to-zip-file}"' \\
     --form 'imageMetadata="{
               \\"api-icon\\" : \\"navigation.jpeg\\",
               \\"api-hero\\": \\"api.svg\\"
            }"
```
