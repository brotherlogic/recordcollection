package com.brotherlogic.recordcollection;

import java.util.Map;
import java.util.TreeMap;

public class EnvironmentVariable {

  private static Map<String, String> localVariables = new TreeMap<String, String>();

  public static void set(String key, String value) {
    localVariables.put(key,value);
  }

  public static void clear() {
    localVariables.clear();
  }
  
  public static String get(String key){
    if (localVariables.containsKey(key)) 
      return localVariables.get(key);
    else
      return System.getenv(key);
  }
}
