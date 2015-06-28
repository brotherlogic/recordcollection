package com.brotherlogic.recordcollection;

import java.io.IOException;
import java.io.StringReader;

import org.junit.Assert;
import org.junit.Test;

public class ConfigTest {
 
    @Test
    public void testConfigCorrectlyParsedFromString() {
	Config c = new Config("ApiKey","ApiSecret",null);
	
	Assert.assertEquals("ApiKey",c.getApiKey());
	Assert.assertEquals("ApiSecret",c.getApiSecret());
        Assert.assertNull(c.getService());
    }
}
