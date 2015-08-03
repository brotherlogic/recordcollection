package com.brotherlogic.recordcollection.integration;

import java.io.BufferedReader;
import java.io.DataOutputStream;
import java.net.HttpURLConnection;
import java.io.InputStreamReader;
import java.io.IOException;
import java.net.URL;

import org.apache.log4j.ConsoleAppender;
import org.apache.log4j.Level;
import org.apache.log4j.Logger;
import org.apache.log4j.PatternLayout;

import com.brotherlogic.recordcollection.TestingSystem;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;

import org.junit.Assert;
import org.junit.Before;
import org.junit.Test;

import org.scribe.model.Token;

public class BaseIntegrationTestIT {
  
  @Before
  public void setup() {
    // Set up logging
    ConsoleAppender console = new ConsoleAppender(); // create appender
    String pattern = "[%c] [%d{dd MMM yyyy - hh:mm:ss}] %5p - %m %n";
    console.setLayout(new PatternLayout(pattern));
    console.setThreshold(Level.DEBUG);
    console.activateOptions();
    
    // add appender to any Logger (here is root)
    Logger.getRootLogger().removeAllAppenders();
    Logger.getRootLogger().addAppender(console);
    
    // Clean out the testing system - add a testing token
    new TestingSystem().getStorage(true).storeToken(new Token("TestToken","TestKey"));
  }

  @Test
  public void testGet() throws IOException {
    String str = get("madeuppath");
    JsonElement json = new JsonParser().parse(str);
    Assert.assertTrue(json.isJsonNull());
  }

  @Test
  public void testPut() throws IOException {
    String str = put("madeuppath", new JsonObject());
    JsonElement json = new JsonParser().parse(str);
    Assert.assertTrue(json.isJsonNull());
  }
  
  public String get(String path) throws IOException {
    URL obj = new URL("http://localhost:8080/endpoint/" + path);
    HttpURLConnection con = (HttpURLConnection) obj.openConnection();
    con.setRequestMethod("GET");
    BufferedReader in = new BufferedReader(new InputStreamReader(con.getInputStream()));
    String inputLine;
    StringBuffer response = new StringBuffer();

    while ((inputLine = in.readLine()) != null) {
      response.append(inputLine);
    }
    in.close();

    
    return response.toString();
  }

  public String put(String path, JsonElement data) throws IOException {
    URL obj = new URL("http://localhost:8080/endpoint/" + path);
    HttpURLConnection con = (HttpURLConnection) obj.openConnection();
    con.setRequestMethod("PUT");
    con.setDoOutput(true);
    DataOutputStream wr = new DataOutputStream(con.getOutputStream());
    wr.writeBytes(data.toString());
    wr.flush();
    wr.close();

    BufferedReader in = new BufferedReader(new InputStreamReader(con.getInputStream()));
    String inputLine;
    StringBuffer response = new StringBuffer();

    while ((inputLine = in.readLine()) != null) {
      response.append(inputLine);
    }
    in.close();

    return response.toString();
  }
}
