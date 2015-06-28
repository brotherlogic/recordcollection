package com.brotherlogic.recordcollection;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.Reader;

import org.scribe.oauth.OAuthService;

public class Config {

    private String key;
    private String secret;
    private OAuthService service;
    
    public Config(String k, String s, OAuthService ser) {
        key = k;
        secret = s;
        service = ser;
    }

    public OAuthService getService() {
        return service;
    }
    
    public String getApiKey() {
	return key;
    }

    public String getApiSecret() {
	return secret;
    }

    public String toString() {
        return super.toString() + ": " + key + "," + secret + "," + service;
    }
}
