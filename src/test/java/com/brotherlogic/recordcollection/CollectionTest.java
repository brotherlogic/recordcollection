package com.brotherlogic.recordcollection;

import java.util.Arrays;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.junit.Assert;
import org.junit.Test;

public class CollectionTest {

  private Logger logger = Logger.getLogger(getClass());

  @Test
  public void testBuildGet() {
    FolderGroup c = new FolderGroup(Arrays.asList(new Integer[] {12,13,14}), Arrays.asList(new Integer[] {10,10}), "TestingCollection");

    Assert.assertEquals(3, c.getFolders().size());
    Assert.assertEquals(2, c.getProps().size());
    Assert.assertEquals("TestingCollection", c.getName());
  }  
}
