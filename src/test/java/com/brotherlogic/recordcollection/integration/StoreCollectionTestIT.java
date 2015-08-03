package com.brotherlogic.recordcollection.integration;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonPrimitive;

import java.io.IOException;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.junit.Assert;
import org.junit.Test;

/**
 * Integration tests to deal with the storing and loading of collections
 */ 
public class StoreCollectionTestIT extends BaseIntegrationTestIT {

  private Logger logger = Logger.getLogger(getClass());
  
  @Test
  public void testStoreAndLoadCollection() throws IOException {
    JsonObject collection = new JsonObject();
    JsonArray folders = new JsonArray();
    folders.add(new JsonPrimitive(12));
    folders.add(new JsonPrimitive(15));
    collection.add("folders",folders);
    JsonArray proportions = new JsonArray();
    proportions.add(new JsonPrimitive(10));
    proportions.add(new JsonPrimitive(10));
    collection.add("proportions", proportions);
    collection.add("name", new JsonPrimitive("TestCollection"));
    
    put("collection?token=TestToken",collection);
    String resp = get("collection?token=TestToken&name=TestCollection");
    logger.log(Level.INFO, "PUT COLLECTION = " + resp);
    JsonElement elem = new JsonParser().parse(resp);
    Assert.assertFalse(elem.isJsonNull());
    JsonObject objResp = elem.getAsJsonObject();
    Assert.assertEquals(collection,objResp);
  }
  
}
