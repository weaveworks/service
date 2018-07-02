var prepopulateFields = {
    creditCardAddress1:"123 Any Street",
    creditCardAddress2:"Suite #999",
    creditCardCountry:"USA",
    creditCardHolderName:"John Doe"
};

var publicIpAddress = "52.211.39.209:8080";
var authTokensAPI = "https://cors-anywhere.herokuapp.com/" + publicIpAddress + "/payments/authTokens";
var updateAccountAPI = "https://cors-anywhere.herokuapp.com/" + publicIpAddress + "/payments";
var createUserAPI = "https://cors-anywhere.herokuapp.com/" + publicIpAddress + "/accounts";
var async = true;

function loadHostedPage(userID) {
    getAuthTokens(userID, renderHPMPage);
}

function getAuthTokens(userID, renderUI) {
    var request = new XMLHttpRequest();
    if (userID == null || userID == "") {
        request.open('GET', authTokensAPI, async);
    } else {
        request.open('GET', authTokensAPI + "/" + userID, async);
    }
    request.onload = request.onload = doOnLoad(request, function(data) {renderUI(data)});
    request.onerror = doOnError;
    request.send();
}

function renderHPMPage(params) {
    params.style = "overlay";
    params.submitEnabled = "true";
    params.locale = "gb_EN";
    console.log("Sending params: ", params);
    var callback;
    if (params.field_accountId == null || params.field_accountId == "") {
        callback = create;
    } else {
        callback = update;
    }
    Z.render(
        params,
        prepopulateFields,
        callback
    );
}

function update(response) {
    if(isHPMOk(response)) {
        console.log("refId="+response.refId);
        performRequest(updateAccountAPI + "/" + response.refId, function() {alert("Account updated!")}, null);
    }
}

function create(response) {
    if(isHPMOk(response)) {
        console.log("RefId ok. Creating new user. " +response.refId);
        var newUserParams = {
            id:            "Test" + Math.random().toString(36).substring(2),
            country:            "United Kingdom",
            currency:           "USD",
            firstName:          "Phil",
            lastName:           "Winder",
            email:              "phil@winderresearch.com",
            paymentMethodId:    response.refId,
            subscriptionPlanId: "2c92c0f9564ef7ba01566b2d1d970bb8"
        };
        console.log("Sending: " + newUserParams);
        performRequest(createUserAPI, function() {alert("Account created!")}, newUserParams);
    }
}

function performRequest(url, onSuccess, body) {
    var request = new XMLHttpRequest();
    request.open('POST', url, async);
    request.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
    request.onload = doOnLoad(request, onSuccess);
    request.onerror = doOnError;
    if (body != null) {
        request.send(JSON.stringify(body));
    } else {
        request.send();
    }
}

function doOnLoad(request, doSomething) {
    return function() {
        if (request.status >= 200 && request.status < 400) {
            var data = JSON.parse(request.responseText);
            console.log(data);
            doSomething(data);
        } else {
            alert(request.status);
        }
    }
}

function doOnError() {
    alert("There was an error");
}

function isHPMOk(response) {
    if (!response.success) {
        alert("errorcode=" + response.errorCode + ", errorMessage=" + response.errorMessage);
    }
    return response.success;
}
