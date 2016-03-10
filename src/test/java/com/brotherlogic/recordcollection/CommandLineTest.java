package com.brotherlogic.recordcollection;

import java.util.Map;
import java.util.TreeMap;

import org.junit.Test;
import org.junit.Assert;

public class CommandLineTest {

  public void storeUserTokenTest() {
    Map<String,String> cMap = new TreeMap<String,String>();
    cMap.put("user","brotherlogic");
    cMap.put("token","blah");

    CommandLineInterface cli = new CommandLineInterface();
    String result = cli.run(cMap);
    Assert.assertEquals(result,"STORED");

    //Re-access
    cMap.remove("token");
    String secondResult = cli.run(cMap);
    Assert.assertNotEquals(result,"NEED_TOKEN");
  }

  public void noStoreUserTokenAccess() {
    Map<String,String> cMap = new TreeMap<String,String>();
    cMap.put("user","brotherlogic");

    CommandLineInterface cli = new CommandLineInterface();
    String result = cli.run(cMap);
    Assert.assertEquals(result,"NEED_TOKEN");
  }

}
