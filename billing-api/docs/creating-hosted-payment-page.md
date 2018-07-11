# Notes on how to create a hosted payment page

## Create the hosted page

1. Create an machine instance in the cloud. Expose port 80 and 8080.
2. Install docker: `wget -qO- https://get.docker.com/ | sh`
3. Get the public IP address
4. Log into the zuora UI
5. Go to Settings->Payments->Setup Hosted Pages
6. Create a new hosted page of type "Credit card"
7. Fill in the details and set the hosted domain to http://YOUR_IP_ADDRESS
8. Leave the callback blank for now (in the real app, you would point this to a receiver page).
9. Click generate and save page at the bottom.
10. Find and copy the generage page ID.

## Create website to host form

[In an older version of this repo there was a demo website that showed you how this API should interact with a UI.]

### Running the API

First run the billing API with the correct settings. See the API documentation. Run the server on port 8080, so the user's browser has access to the API (DON'T DO THIS IN PRODUCTION).

### Running the webserver

First edit the files so that they contain your unique settings. E.g. IP addresses and HPM page ID. Note that the javascript code uses a cors proxy to get around cors issues when the users browser attempts to connect to the API (which is on a different port when compared to the webapp).

Copy the hpm_demo files to the remote host. Then CD into the directory and build and run the web app. 

```
sudo docker rm -f app ; sudo docker build -t app . ; sudo docker run -d -p 80:80 --name app app
```

## Usage

1. Browse to the public ip address. Open the developer console. Click create or update.

2. Enter some dummy details.

3. Press submit.

A refId will be returned which uniquely determines the new payment method. This is then sent back to the API.

## Video demo

You can see this in action here:

[![HPM create/update demo](http://img.youtube.com/vi/Zytj4FJg-nI/0.jpg)](https://youtu.be/Zytj4FJg-nI "HPM Demo")
