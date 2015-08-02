package com.brotherlogic.recordcollection;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.Reader;

public class Config {

    private String key;
    private String secret;
    private DiscogsService service;
    
    public Config(String k, String s, DiscogsService ser) {
        key = k;
        secret = s;
        service = ser;
    }

    public DiscogsService getService() {
        return service;
    }
    
    public String getApiKey() {
	return key;
    }

    public String getApiSecret() {
	return secret;
    }
}
